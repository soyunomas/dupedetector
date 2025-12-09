package scanner

import (
	"fmt"
	"io/fs"
//	"os"
	"path/filepath"
	"syscall"

	"github.com/soyunomas/dupedetector/internal/entities"
)

// Config define las reglas para el escaneo.
type Config struct {
	MinSize   int64    // Tamaño mínimo en bytes para considerar
	Excludes  []string // Lista de carpetas a ignorar (ej: ".git", "node_modules")
}

// FileScanner encapsula la lógica de recorrido del sistema de archivos.
type FileScanner struct {
	cfg Config
}

// New crea una nueva instancia del escáner con configuración.
func New(cfg Config) *FileScanner {
	return &FileScanner{cfg: cfg}
}

// Scan recorre rootDir y devuelve un mapa inicial agrupado por tamaño.
// Map: [Tamaño] -> [Grupo de Archivos]
func (s *FileScanner) Scan(rootDir string) (map[int64]*entities.FileGroup, error) {
	// Inicializamos el mapa. Usamos punteros para evitar copias de memoria innecesarias.
	filesBySize := make(map[int64]*entities.FileGroup)

	fmt.Printf("🔍 Iniciando escaneo en: %s\n", rootDir)

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		// 1. Manejo de errores de acceso (permisos, etc)
		if err != nil {
			// En una app robusta, logueamos el error y continuamos, no paramos todo.
			// fmt.Fprintf(os.Stderr, "⚠️ Error accediendo a %s: %v\n", path, err)
			return nil 
		}

		// 2. Si es directorio, verificamos si debemos ignorarlo
		if d.IsDir() {
			if s.shouldIgnoreDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// 3. Obtener información del archivo (Stat)
		info, err := d.Info()
		if err != nil {
			return nil
		}

		// 4. Filtro de Tamaño
		size := info.Size()
		if size < s.cfg.MinSize {
			return nil
		}

		// 5. Construcción de la Entidad
		// Extraemos Inode/Device de forma específica según el OS (syscall)
		devID, inode := getSysInfo(info)

		fileEntity := &entities.FileInfo{
			Path:     path,
			Size:     size,
			ModTime:  info.ModTime(),
			DeviceID: devID,
			Inode:    inode,
			// Hash se calculará en la siguiente fase (Fase 2), aquí lo dejamos en 0
		}

		// 6. Agrupar
		if _, exists := filesBySize[size]; !exists {
			filesBySize[size] = &entities.FileGroup{}
		}
		filesBySize[size].Add(fileEntity)

		return nil
	})

	return filesBySize, err
}

// shouldIgnoreDir verifica si la carpeta está en la lista de exclusión.
func (s *FileScanner) shouldIgnoreDir(name string) bool {
	// Implementación básica O(N). Para muchas exclusiones, usar un map[string]bool sería O(1).
	for _, excl := range s.cfg.Excludes {
		if name == excl {
			return true
		}
	}
	return false
}

// getSysInfo extrae DeviceID e Inode de forma "segura".
// Nota: Sys() devuelve interface{}, necesitamos aserción de tipos.
func getSysInfo(info fs.FileInfo) (uint64, uint64) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0 // No soportado (ej. Windows en algunos contextos), pero no rompemos.
	}
	// Conversión segura para compatibilidad multi-arquitectura
	return uint64(stat.Dev), uint64(stat.Ino)
}
