package entities

import (
	"time"
)

// FileInfo representa un archivo en disco con los metadatos necesarios.
// Añadimos tags `json` para serialización.
type FileInfo struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size_bytes"`
	Hash     uint64    `json:"hash"`
	ModTime  time.Time `json:"mod_time"`
	DeviceID uint64    `json:"device_id"`
	Inode    uint64    `json:"inode"`
}

// FileGroup representa un conjunto de archivos que comparten criterios.
type FileGroup struct {
	Count int64       `json:"count"`
	Files []*FileInfo `json:"files"`
}

// Add agrega un archivo al grupo
func (fg *FileGroup) Add(f *FileInfo) {
	fg.Files = append(fg.Files, f)
	fg.Count++
}
