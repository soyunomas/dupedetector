package hasher

import (
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
)

// BlockSize optimiza la lectura del disco (32KB es un buen estándar)
const BlockSize = 32 * 1024

// PreHashSize define cuánto leemos para la prueba rápida (4KB)
const PreHashSize = 4 * 1024

// HashFile calcula el hash completo (xxHash64)
func HashFile(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	h := xxhash.New()
	// io.CopyBuffer usa un buffer reutilizable para menos allocs
	if _, err := io.CopyBuffer(h, file, make([]byte, BlockSize)); err != nil {
		return 0, err
	}

	return h.Sum64(), nil
}

// HashFirstBlock calcula el hash SOLO de los primeros 4KB.
// Es una prueba rápida para descartar diferencias obvias.
func HashFirstBlock(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	h := xxhash.New()
	
	// LimitReader asegura que solo leemos hasta PreHashSize
	limitReader := io.LimitReader(file, PreHashSize)
	
	if _, err := io.Copy(h, limitReader); err != nil {
		return 0, err
	}

	return h.Sum64(), nil
}
