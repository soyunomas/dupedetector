package engine

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/soyunomas/dupedetector/internal/entities"
	"github.com/soyunomas/dupedetector/internal/hasher"
	"github.com/soyunomas/dupedetector/internal/scanner"
)

// Definimos las estrategias de conservación disponibles
type KeepStrategy int

const (
	KeepShortestPath KeepStrategy = iota // Default
	KeepLongestPath
	KeepOldest
	KeepNewest
)

type Options struct {
	MinSize  int64
	Excludes []string
	Strategy KeepStrategy
}

type Stats struct {
	TotalFilesScanned int64
	FilesByHash       map[uint64]*entities.FileGroup
	DuplicatesCount   int64
	Duration          time.Duration
}

type Runner struct {
	opts Options
}

func New(opts Options) *Runner {
	return &Runner{opts: opts}
}

func (r *Runner) Run(rootDir string) (*Stats, error) {
	start := time.Now()

	// --- PASO 1: SCANNER ---
	fmt.Println("🔍 Fase 1: Escaneando sistema de archivos...")
	sc := scanner.New(scanner.Config{
		MinSize:  r.opts.MinSize,
		Excludes: r.opts.Excludes,
	})

	filesBySize, err := sc.Scan(rootDir)
	if err != nil {
		return nil, fmt.Errorf("fallo en scanner: %w", err)
	}

	var initialCandidates []string
	var totalScanned int64
	for _, group := range filesBySize {
		totalScanned += group.Count
		if group.Count > 1 {
			for _, f := range group.Files {
				initialCandidates = append(initialCandidates, f.Path)
			}
		}
	}
	fmt.Printf("   -> %d archivos encontrados. %d candidatos por tamaño.\n", totalScanned, len(initialCandidates))

	// --- PASO 2: PRE-HASHING ---
	fmt.Println("🔍 Fase 2: Pre-Hashing (4KB check)...")
	preHashGroups := r.processPreHash(initialCandidates)

	var finalCandidates []string
	for _, paths := range preHashGroups {
		if len(paths) > 1 {
			finalCandidates = append(finalCandidates, paths...)
		}
	}
	fmt.Printf("\n   -> %d candidatos tras Pre-Hash.\n", len(finalCandidates))

	// --- PASO 3: FULL HASHING ---
	fmt.Println("🔍 Fase 3: Hashing Completo (Verificación final)...")
	finalGroups := r.processFullHash(finalCandidates)
	fmt.Println("\n   -> Hashing terminado.")

	// --- PASO 4: ORDENAR Y FINALIZAR ---
	sortGroups(finalGroups, r.opts.Strategy)

	var dupesCount int64
	for _, group := range finalGroups {
		if group.Count > 1 {
			dupesCount += group.Count - 1
		}
	}

	return &Stats{
		TotalFilesScanned: totalScanned,
		FilesByHash:       finalGroups,
		DuplicatesCount:   dupesCount,
		Duration:          time.Since(start),
	}, nil
}

// processPreHash: Optimizada para velocidad bruta.
func (r *Runner) processPreHash(paths []string) map[uint64][]string {
	type job struct{ path string }
	type result struct {
		path string
		hash uint64
		err  error
	}

	// Restauramos buffer completo para evitar bloqueo de workers
	jobs := make(chan job, len(paths))
	results := make(chan result, len(paths))

	numWorkers := runtime.NumCPU()
	var wg sync.WaitGroup

	// Workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				h, err := hasher.HashFirstBlock(j.path)
				results <- result{j.path, h, err}
			}
		}()
	}

	// Llenamos la cola a máxima velocidad
	for _, p := range paths {
		jobs <- job{p}
	}
	close(jobs)

	// Monitor de cierre
	go func() {
		wg.Wait()
		close(results)
	}()

	groups := make(map[uint64][]string)
	processed := 0
	
	// Consumidor sin bloqueos
	for res := range results {
		processed++
		if processed%200 == 0 { // Menos I/O a consola
			fmt.Print(".")
		}
		if res.err == nil {
			groups[res.hash] = append(groups[res.hash], res.path)
		}
	}
	return groups
}

// processFullHash: Workers + Stat eficiente
func (r *Runner) processFullHash(paths []string) map[uint64]*entities.FileGroup {
	type job struct{ path string }
	type result struct {
		path  string
		hash  uint64
		stats hasher.FileStats
		err   error
	}

	// Restauramos buffer completo
	jobs := make(chan job, len(paths))
	results := make(chan result, len(paths))

	numWorkers := runtime.NumCPU()
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				h, stats, err := hasher.HashFile(j.path)
				results <- result{j.path, h, stats, err}
			}
		}()
	}

	for _, p := range paths {
		jobs <- job{p}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	groups := make(map[uint64]*entities.FileGroup)
	processed := 0

	for res := range results {
		processed++
		if processed%50 == 0 { // Menos print para no saturar stdout
			fmt.Print("#")
		}

		if res.err != nil {
			continue
		}
		if _, exists := groups[res.hash]; !exists {
			groups[res.hash] = &entities.FileGroup{}
		}

		// Stat solo si es estrictamente necesario para la estrategia
		var modTime time.Time
		if r.opts.Strategy == KeepOldest || r.opts.Strategy == KeepNewest {
			if info, err := os.Stat(res.path); err == nil {
				modTime = info.ModTime()
			}
		}

		groups[res.hash].Add(&entities.FileInfo{
			Path:     res.path,
			Hash:     res.hash,
			Size:     res.stats.Size,
			DeviceID: res.stats.DeviceID,
			Inode:    res.stats.Inode,
			ModTime:  modTime,
		})
	}
	return groups
}
