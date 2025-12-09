# Variables del proyecto
BINARY_NAME=dupedetector
CMD_PATH=cmd/dupedetector/main.go

# Flags de compilación: 
# -s: Omitir tabla de símbolos (menor tamaño)
# -w: Omitir información de depuración DWARF (menor tamaño)
LDFLAGS=-ldflags "-s -w"

.PHONY: all build run test clean tidy help

# Meta por defecto
all: build

# 🔨 Compilar el binario optimizado
build:
	@echo "🔨 Compilando $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) $(CMD_PATH)
	@echo "✅ Binario generado: ./$(BINARY_NAME)"

# 🚀 Compilar y ejecutar (prueba rápida en directorio actual)
run: build
	@echo "🚀 Ejecutando en ./"
	./$(BINARY_NAME) -dir .

# 🧪 Ejecutar tests (si los hubiera)
test:
	@echo "🧪 Ejecutando tests unitarios..."
	go test -v ./...

# 🧹 Limpieza profunda
clean:
	@echo "🧹 Limpiando artefactos..."
	rm -f $(BINARY_NAME)
	rm -f *.sh              # Borra scripts generados por -output
	rm -rf TRASH_BIN        # Borra la carpeta de basura de pruebas
	go clean
	@echo "✨ Limpio."

# 📦 Gestionar dependencias (go.mod)
tidy:
	@echo "📦 Ordenando módulos..."
	go mod tidy

# ℹ️ Ayuda
help:
	@echo "Comandos disponibles:"
	@echo "  make build   - Compila el binario optimizado"
	@echo "  make run     - Compila y ejecuta en el directorio actual"
	@echo "  make test    - Ejecuta tests"
	@echo "  make clean   - Borra binarios, scripts .sh y TRASH_BIN"
	@echo "  make tidy    - Actualiza go.mod y go.sum"
