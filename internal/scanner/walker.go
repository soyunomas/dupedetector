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
	Excludes  []string // Lista de carpetas a ignorar
}

// FileScanner encapsula la lógica de recorrido del sistema de archivos.
type FileScanner struct {
	cfg        Config
	excludeMap map[string]struct{} // Optimización O(1)
}

// New crea una nueva instancia del escáner con configuración.
func New(cfg Config) *FileScanner {
	// Pre-procesamos excludes a un mapa para búsquedas instantáneas
	exMap := make(map[string]struct{}, len(cfg.Excludes))
	for _, e := range cfg.Excludes {
		exMap[e] = struct{}{}
	}

	return &FileScanner{
		cfg:        cfg,
		excludeMap: exMap,
	}
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
			return nil 
		}

		// 2. Si es directorio, verificamos si debemos ignorarlo (Optimizado)
		if d.IsDir() {
			if _, ok := s.excludeMap[d.Name()]; ok {
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
			// Hash se calculará en la siguiente fase (Fase 2)
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

// getSysInfo extrae DeviceID e Inode de forma "segura".
func getSysInfo(info fs.FileInfo) (uint64, uint64) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0 
	}
	return uint64(stat.Dev), uint64(stat.Ino)
}
