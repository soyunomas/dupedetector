package engine

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/soyunomas/dupedetector/internal/entities"
	"github.com/soyunomas/dupedetector/internal/hasher"
	"github.com/soyunomas/dupedetector/internal/scanner"
)

// Definimos las estrategias de conservación disponibles
type KeepStrategy int

const (
	KeepShortestPath KeepStrategy = iota // Default: Ruta más corta ("/a" gana a "/copy/a")
	KeepLongestPath                      // Ruta más larga gana
	KeepOldest                           // Fecha de modificación más antigua gana
	KeepNewest                           // Fecha de modificación más reciente gana
)

type Options struct {
	MinSize  int64
	Excludes []string
	Strategy KeepStrategy // Nueva opción de configuración
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
	// Pasamos la estrategia elegida por el usuario
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

// processPreHash ejecuta el hashing parcial
func (r *Runner) processPreHash(paths []string) map[uint64][]string {
	type job struct{ path string }
	type result struct {
		path string
		hash uint64
		err  error
	}

	jobs := make(chan job, len(paths))
	results := make(chan result, len(paths))

	numWorkers := runtime.NumCPU()
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			h, err := hasher.HashFirstBlock(j.path)
			results <- result{j.path, h, err}
		}
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker()
	}

	for _, p := range paths {
		jobs <- job{p}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	groups := make(map[uint64][]string)
	processed := 0
	for res := range results {
		processed++
		if processed%100 == 0 {
			fmt.Print(".")
		}
		if res.err == nil {
			groups[res.hash] = append(groups[res.hash], res.path)
		}
	}
	return groups
}

// processFullHash ejecuta el hashing completo
func (r *Runner) processFullHash(paths []string) map[uint64]*entities.FileGroup {
	type job struct{ path string }
	type result struct {
		path string
		hash uint64
		err  error
	}

	jobs := make(chan job, len(paths))
	results := make(chan result, len(paths))

	numWorkers := runtime.NumCPU()
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			h, err := hasher.HashFile(j.path)
			results <- result{j.path, h, err}
		}
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker()
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
		if processed%10 == 0 {
			fmt.Print("#")
		}

		if res.err != nil {
			continue
		}
		if _, exists := groups[res.hash]; !exists {
			groups[res.hash] = &entities.FileGroup{}
		}

		var size int64
		var devID, inode uint64

		if info, err := os.Stat(res.path); err == nil {
			size = info.Size()
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				devID = uint64(stat.Dev)
				inode = uint64(stat.Ino)
			}
		}

		groups[res.hash].Add(&entities.FileInfo{
			Path:     res.path,
			Hash:     res.hash,
			Size:     size,
			DeviceID: devID,
			Inode:    inode,
			// Importante: scanner no setea ModTime, lo recuperamos aquí o usamos Stat arriba
			// Para optimizar, asumimos que se necesita ModTime para el sorter "oldest/newest".
			// Stat ya nos dio 'info', extraemos ModTime.
		})
		
		// Un pequeño fix: Recuperar ModTime si no venía del scanner o si queremos precisión fresca
		if info, err := os.Stat(res.path); err == nil {
			groups[res.hash].Files[len(groups[res.hash].Files)-1].ModTime = info.ModTime()
		}
	}
	return groups
}
