package hasher

import (
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/cespare/xxhash/v2"
)

// BlockSize optimiza la lectura del disco (32KB es un buen estándar)
const BlockSize = 32 * 1024

// PreHashSize define cuánto leemos para la prueba rápida (4KB)
const PreHashSize = 4 * 1024

// bufferPool solo para cargas pesadas (HashFile completo)
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, BlockSize)
		return &b
	},
}

// hashPool para reutilizar el estado del digest
var hashPool = sync.Pool{
	New: func() any {
		return xxhash.New()
	},
}

type FileStats struct {
	Size     int64
	DeviceID uint64
	Inode    uint64
}

// HashFile calcula el hash completo. Aquí SI vale la pena usar Pools.
func HashFile(path string) (uint64, FileStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, FileStats{}, err
	}
	defer file.Close()

	// Obtener stats del descriptor abierto (Rápido)
	info, err := file.Stat()
	if err != nil {
		return 0, FileStats{}, err
	}

	stats := FileStats{Size: info.Size()}
	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		stats.DeviceID = uint64(sys.Dev)
		stats.Inode = uint64(sys.Ino)
	}

	// Pooling
	h := hashPool.Get().(*xxhash.Digest)
	h.Reset()
	defer hashPool.Put(h)

	bufPtr := bufferPool.Get().(*[]byte)
	buf := *bufPtr
	defer bufferPool.Put(bufPtr)

	if _, err := io.CopyBuffer(h, file, buf); err != nil {
		return 0, stats, err
	}

	return h.Sum64(), stats, nil
}

// HashFirstBlock optimizado para baja latencia.
// NO usa sync.Pool de buffers para evitar contención en lecturas pequeñas.
func HashFirstBlock(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	h := hashPool.Get().(*xxhash.Digest)
	h.Reset()
	defer hashPool.Put(h)

	// Alloc simple de 4KB. Es barato y evita locking del Pool global.
	// Usamos ReadFull para asegurar consistencia.
	buf := make([]byte, PreHashSize)
	n, err := io.ReadFull(file, buf)
	
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return 0, err
	}

	// Hash de lo que se haya podido leer
	_, _ = h.Write(buf[:n])

	return h.Sum64(), nil
}
