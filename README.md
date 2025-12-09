# dupedetector

Una herramienta CLI para encontrar y gestionar archivos duplicados en el sistema de archivos. Utiliza una estrategia de hashing escalonado para identificar copias exactas y ofrece varios métodos seguros para eliminarlas.

## Características

*   **Detección en 3 Fases:** Agrupación por tamaño -> Hash parcial (4KB) -> Hash completo (xxHash64).
*   **Concurrencia:** Procesamiento paralelo de archivos.
*   **Detección de Hard Links:** Distingue entre copias físicas (que ocupan espacio) y referencias de inodos (que no).
*   **Estrategias de Conservación:** Permite elegir qué archivo mantener (`shortest`, `longest`, `newest`, `oldest`).
*   **Modos de Borrado:**
    *   `Dry Run` (Por defecto): Solo reporta.
    *   `-trash`: Mueve duplicados a una carpeta temporal (`TRASH_BIN`).
    *   `-output`: Genera un script shell para revisión manual.
    *   `-delete`: Eliminación directa.
*   **Integración:** Salida JSON opcional para scripts externos.

## Instalación

Necesitas tener Go instalado.

```bash
git clone https://github.com/soyunomas/dupedetector.git
cd dupedetector
make build
```

Esto generará el binario `dupedetector` en la raíz.

## Uso

### Escaneo básico (Modo seguro)
Solo muestra los duplicados encontrados sin borrar nada.

```bash
./dupedetector -dir /ruta/a/escanear
```

### Estrategias de Conservación (`-keep`)
Define qué archivo se considera el "Original" (Keeper) y cuáles se marcan para borrar.

*   `shortest`: Mantiene la ruta más corta (Default).
*   `longest`: Mantiene la ruta más larga.
*   `newest`: Mantiene el archivo modificado más recientemente.
*   `oldest`: Mantiene el archivo más antiguo.

```bash
# Ejemplo: Mantener la versión más nueva, borrar las viejas
./dupedetector -dir ~/Fotos -keep newest
```

### Opciones de Limpieza

#### 1. Mover a Papelera (Recomendado)
Mueve los duplicados a una carpeta `./TRASH_BIN` en el directorio de ejecución, renombrándolos para evitar colisiones.

```bash
./dupedetector -dir ~/Descargas -trash
```

#### 2. Generar Script de Borrado
Crea un archivo `.sh` con los comandos `rm` para que puedas revisarlos antes de ejecutar.

```bash
./dupedetector -dir ~/Descargas -output limpiar.sh
# Luego revisas y ejecutas:
# sh limpiar.sh
```

#### 3. Borrado Directo
Elimina los archivos inmediatamente.

```bash
./dupedetector -dir ~/Descargas -delete
```

### Salida JSON
Para integración con otras herramientas.

```bash
./dupedetector -dir . -json > reporte.json
```

## Flags disponibles

| Flag | Descripción | Default |
|------|-------------|---------|
| `-dir` | Directorio raíz a escanear | `.` |
| `-min-size` | Tamaño mínimo de archivo en bytes | `1024` |
| `-keep` | Criterio para mantener original (`shortest`, `longest`, `newest`, `oldest`) | `shortest` |
| `-trash` | Mueve duplicados a `./TRASH_BIN` | `false` |
| `-output` | Genera un script `.sh` con comandos de borrado | `""` |
| `-delete` | Borra archivos inmediatamente (Irreversible) | `false` |
| `-json` | Imprime resultado en formato JSON | `false` |

## Licencia

MIT
