package engine

import (
	"sort"

	"github.com/soyunomas/dupedetector/internal/entities"
)

// sortGroups organiza los archivos dentro de cada grupo según la estrategia.
// El objetivo es que el archivo en la posición [0] sea el "Keeper" (Original).
func sortGroups(groups map[uint64]*entities.FileGroup, strategy KeepStrategy) {
	for _, group := range groups {
		if group.Count < 2 {
			continue
		}

		// Ordenamos el slice de archivos.
		// Si la función retorna TRUE, 'i' se coloca antes que 'j' (índice menor).
		sort.Slice(group.Files, func(i, j int) bool {
			f1 := group.Files[i]
			f2 := group.Files[j]

			switch strategy {
			
			case KeepShortestPath:
				// [0] debe ser el más corto
				if len(f1.Path) != len(f2.Path) {
					return len(f1.Path) < len(f2.Path)
				}

			case KeepLongestPath:
				// [0] debe ser el más largo
				if len(f1.Path) != len(f2.Path) {
					return len(f1.Path) > len(f2.Path)
				}

			case KeepOldest:
				// [0] debe ser el más viejo (Fecha menor)
				if !f1.ModTime.Equal(f2.ModTime) {
					return f1.ModTime.Before(f2.ModTime)
				}

			case KeepNewest:
				// [0] debe ser el más nuevo (Fecha mayor)
				if !f1.ModTime.Equal(f2.ModTime) {
					return f1.ModTime.After(f2.ModTime)
				}
			}

			// --- CRITERIOS DE DESEMPATE (Tie-Breakers) ---
			// Si las reglas principales (longitud o fecha) son iguales,
			// necesitamos un determinismo absoluto.
			
			// 1. Longitud de ruta (si no fue el criterio principal)
			if len(f1.Path) != len(f2.Path) {
				if strategy == KeepLongestPath {
					return len(f1.Path) > len(f2.Path)
				}
				return len(f1.Path) < len(f2.Path)
			}

			// 2. Alfabético (último recurso)
			return f1.Path < f2.Path
		})
	}
}
