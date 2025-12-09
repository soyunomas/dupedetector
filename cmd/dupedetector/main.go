package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/soyunomas/dupedetector/internal/engine"
	"github.com/soyunomas/dupedetector/internal/entities"
	"github.com/soyunomas/dupedetector/internal/utils"
)

// --- ESTRUCTURAS PARA EL REPORTE FINAL ---

type Report struct {
	Summary  Summary       `json:"summary"`
	Groups   []GroupResult `json:"groups"`
	Metadata Metadata      `json:"metadata"`
}

type Metadata struct {
	ScannedPath string    `json:"scanned_path"`
	Strategy    string    `json:"strategy"`
	Timestamp   time.Time `json:"timestamp"`
	Duration    string    `json:"duration_human"`
}

type Summary struct {
	TotalFilesScanned int64  `json:"total_files_scanned"`
	TotalDuplicates   int64  `json:"total_duplicates"`
	TotalHardLinks    int64  `json:"total_hard_links"`
	BytesSaved        int64  `json:"bytes_saved"`
	BytesSavedHuman   string `json:"bytes_saved_human"`
}

type GroupResult struct {
	Hash      uint64             `json:"hash"`
	Size      int64              `json:"file_size"`
	Keeper    *entities.FileInfo `json:"keeper"`
	Victims   []Victim           `json:"victims"`
	HardLinks []string           `json:"hardlinks"`
}

type Victim struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type sysID struct {
	dev, inode uint64
}

func main() {
	// Flags
	dirPtr := flag.String("dir", ".", "Directorio a escanear")
	minSizePtr := flag.Int64("min-size", 1024, "Tamaño mínimo en bytes")
	deletePtr := flag.Bool("delete", false, "⚠️  BORRADO NUCLEAR: Elimina archivos inmediatamente")
	trashPtr := flag.Bool("trash", false, "♻️  SOFT DELETE: Mueve archivos a una carpeta ./TRASH_BIN")
	keepPtr := flag.String("keep", "shortest", "Criterio: shortest, longest, oldest, newest")
	jsonPtr := flag.Bool("json", false, "Salida en formato JSON a stdout")
	outputPtr := flag.String("output", "", "Genera un script .sh")

	flag.Parse()

	// Validación de flags incompatibles
	actionCount := 0
	if *deletePtr { actionCount++ }
	if *trashPtr { actionCount++ }
	if *outputPtr != "" { actionCount++ }

	if actionCount > 1 {
		fmt.Fprintln(os.Stderr, "❌ Error: Solo puedes elegir UNA acción: -delete, -trash, o -output")
		os.Exit(1)
	}

	// 1. Configurar Estrategia
	var strategy engine.KeepStrategy
	switch strings.ToLower(*keepPtr) {
	case "shortest": strategy = engine.KeepShortestPath
	case "longest":  strategy = engine.KeepLongestPath
	case "oldest":   strategy = engine.KeepOldest
	case "newest":   strategy = engine.KeepNewest
	default:
		fmt.Fprintf(os.Stderr, "❌ Estrategia desconocida: %s\n", *keepPtr)
		os.Exit(1)
	}

	// 2. Ejecutar Engine
	opts := engine.Options{
		MinSize:  *minSizePtr,
		Excludes: []string{".git", "node_modules", ".DS_Store", "TRASH_BIN"}, // Excluir nuestra propia basura
		Strategy: strategy,
	}
	runner := engine.New(opts)

	if !*jsonPtr {
		fmt.Printf("🚀 Dupedetector v1.1 - Escaneando: %s\n", *dirPtr)
		fmt.Printf("⚖️  Estrategia: Mantener %s\n", strings.ToUpper(*keepPtr))
		fmt.Println("------------------------------------------------")
	}

	stats, err := runner.Run(*dirPtr)
	if err != nil {
		die(err, *jsonPtr)
	}

	// 3. Generar Reporte
	report := generateReport(stats, *dirPtr, *keepPtr)

	// 4. Salida
	if *jsonPtr {
		printJSON(report)
		return
	}

	if *outputPtr != "" {
		if err := generateShellScript(report, *outputPtr); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error generando script: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n📄 Script generado: %s\n", *outputPtr)
		return
	}

	// Acción Directa (Texto, Delete o Trash)
	processResults(report, *deletePtr, *trashPtr)
}

// processResults maneja la visualización y las acciones inmediatas (delete/trash)
func processResults(r Report, deleteMode, trashMode bool) {
	if len(r.Groups) == 0 {
		fmt.Println("✅ ¡Limpio! No se encontraron duplicados.")
		return
	}

	// Preparar carpeta de basura si es necesario
	trashDir := "TRASH_BIN"
	if trashMode {
		if err := os.MkdirAll(trashDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error creando carpeta de basura: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("♻️  Modo Papelera: Los archivos se moverán a ./%s/\n", trashDir)
	} else if deleteMode {
		fmt.Println("🔥 MODO DESTRUCTIVO: Los archivos se borrarán para siempre.")
	}

	fmt.Println("🔴 DUPLICADOS ENCONTRADOS:")
	actionCount := 0

	for _, g := range r.Groups {
		fmt.Printf("   📦 Grupo (Size: %s) | 👑 KEEPER: %s\n", utils.ByteCountDecimal(g.Size), g.Keeper.Path)
		
		for _, hl := range g.HardLinks {
			fmt.Printf("      🔗 [HardLink]: %s (0B)\n", hl)
		}

		for _, v := range g.Victims {
			if deleteMode {
				// BORRADO NUCLEAR
				if err := os.Remove(v.Path); err != nil {
					fmt.Printf("      ❌ Error borrando %s: %v\n", v.Path, err)
				} else {
					fmt.Printf("      🔥 Borrado: %s\n", v.Path)
					actionCount++
				}
			} else if trashMode {
				// MOVIMIENTO A PAPELERA
				if err := moveToTrash(v.Path, trashDir); err != nil {
					fmt.Printf("      ❌ Error moviendo %s: %v\n", v.Path, err)
				} else {
					fmt.Printf("      ♻️  Movido a basura: %s\n", v.Path)
					actionCount++
				}
			} else {
				// DRY RUN
				fmt.Printf("      🗑️  [Candidato]: %s\n", v.Path)
			}
		}
		fmt.Println("")
	}

	fmt.Println("------------------------------------------------")
	if deleteMode || trashMode {
		fmt.Printf("🏁 Operación completada. Archivos procesados: %d\n", actionCount)
		fmt.Printf("💾 Espacio liberado: %s\n", r.Summary.BytesSavedHuman)
	} else {
		fmt.Printf("🏁 Escaneo terminado. Candidatos a borrar: %d\n", r.Summary.TotalDuplicates)
		fmt.Printf("💾 Espacio recuperable: %s\n", r.Summary.BytesSavedHuman)
		fmt.Println("💡 Opciones disponibles:")
		fmt.Println("   -trash   -> Mover a carpeta segura")
		fmt.Println("   -output  -> Generar script de revisión")
		fmt.Println("   -delete  -> Borrar inmediatamente")
	}
}

// moveToTrash mueve el archivo a la carpeta trashDir.
// Renombra el archivo para evitar colisiones: nombre_TIMESTAMP.ext
func moveToTrash(srcPath, trashDir string) error {
	filename := filepath.Base(srcPath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	// Generar nombre único: archivo_171562912.txt
	uniqueName := fmt.Sprintf("%s_%d%s", nameWithoutExt, time.Now().UnixNano(), ext)
	destPath := filepath.Join(trashDir, uniqueName)

	// Intentar mover (Rename es atómico dentro del mismo FS)
	err := os.Rename(srcPath, destPath)
	if err != nil {
		// Si falla (ej: diferentes particiones), hacemos Copy + Remove
		// Nota: os.Rename falla entre discos distintos.
		if isCrossDeviceError(err) {
			return moveCrossDevice(srcPath, destPath)
		}
		return err
	}
	return nil
}

// isCrossDeviceError detecta si el error es "invalid cross-device link"
func isCrossDeviceError(err error) bool {
	// Es una forma simplificada de detectarlo
	return strings.Contains(err.Error(), "cross-device") || strings.Contains(err.Error(), "EXDEV")
}

// moveCrossDevice copia y borra (para mover entre particiones)
func moveCrossDevice(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}

	// Cerrar explícitamente para asegurar flush
	output.Close()
	input.Close()

	return os.Remove(src)
}

// generateReport, generateShellScript, printJSON, die... (MISMOS QUE ANTES)
func generateReport(stats *engine.Stats, rootDir, strategy string) Report {
	rep := Report{
		Metadata: Metadata{
			ScannedPath: rootDir,
			Strategy:    strategy,
			Timestamp:   time.Now(),
			Duration:    stats.Duration.String(),
		},
		Summary: Summary{
			TotalFilesScanned: stats.TotalFilesScanned,
		},
		Groups: []GroupResult{},
	}

	for _, group := range stats.FilesByHash {
		if group.Count < 2 {
			continue
		}

		keeper := group.Files[0]
		gRes := GroupResult{
			Hash:   keeper.Hash,
			Size:   keeper.Size,
			Keeper: keeper,
		}

		seenInodes := make(map[sysID]bool)
		seenInodes[sysID{keeper.DeviceID, keeper.Inode}] = true

		for _, file := range group.Files[1:] {
			id := sysID{file.DeviceID, file.Inode}
			
			if seenInodes[id] {
				gRes.HardLinks = append(gRes.HardLinks, file.Path)
				rep.Summary.TotalHardLinks++
			} else {
				gRes.Victims = append(gRes.Victims, Victim{
					Path: file.Path,
					Size: file.Size,
				})
				rep.Summary.TotalDuplicates++
				rep.Summary.BytesSaved += file.Size
				seenInodes[id] = true
			}
		}

		if len(gRes.Victims) > 0 || len(gRes.HardLinks) > 0 {
			rep.Groups = append(rep.Groups, gRes)
		}
	}

	rep.Summary.BytesSavedHuman = utils.ByteCountDecimal(rep.Summary.BytesSaved)
	return rep
}

func generateShellScript(r Report, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "#!/bin/sh\n")
	fmt.Fprintf(w, "# Generado por Dupedetector\n")
	fmt.Fprintf(w, "echo 'Iniciando limpieza...'\n\n")

	for _, g := range r.Groups {
		if len(g.Victims) == 0 { continue }
		fmt.Fprintf(w, "# Group Hash: %x\n", g.Hash)
		fmt.Fprintf(w, "# Keeper: %s\n", g.Keeper.Path)
		for _, v := range g.Victims {
			fmt.Fprintf(w, "rm -v %q\n", v.Path)
		}
		fmt.Fprintf(w, "\n")
	}
	return w.Flush()
}

func printJSON(r Report) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(r)
}

func die(err error, jsonMode bool) {
	if jsonMode {
		fmt.Printf(`{"error": "%v"}`+"\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "❌ Error fatal: %v\n", err)
	}
	os.Exit(1)
}
