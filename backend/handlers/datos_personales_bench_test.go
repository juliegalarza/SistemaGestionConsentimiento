// backend/handlers/datos_personales_bench_test.go
package handlers

import (
	"context"
	"os"
	"testing"

	"backend/db"
	"backend/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMain inicializa las conexiones a las dos bases de datos antes de correr los Benchmarks.
func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		panic("DATABASE_URL no definido")
	}

	// Pool para políticas (construirPoliticaDinamica usa db.Pool.Query)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		panic("no se pudo conectar a la BD de políticas: " + err.Error())
	}
	db.Pool = pool

	// ConnDatos para los datos personales
	conn, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		panic("no se pudo conectar a la BD de datos personales: " + err.Error())
	}
	db.ConnDatos = conn

	// Ejecutar Benchmarks
	code := m.Run()

	// Cerrar conexiones
	pool.Close()
	conn.Close()

	os.Exit(code)
}

// Datos de ejemplo para todos los benchmarks
var ejemplo = DatosPersonalesInput{
	IDUsuario:       2,
	Telefono:        "0998123456",
	Celular:         "0987654321",
	Direccion:       "Av. Siempre Viva 742",
	Ciudad:          "Quito",
	Provincia:       "Pichincha",
	FechaNacimiento: "1990-01-01",
	Genero:          "F",
	EstadoCivil:     "Soltera",
}

// BenchmarkCrearEscenarioA mide: construir política, cifrar los 8 campos y
// luego hacer INSERT ON CONFLICT (UPSERT) en datos_personales.
func BenchmarkCrearEscenarioA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// 1) Construir política ABE dinámica
		politica, err := construirPoliticaDinamica(ejemplo.IDUsuario)
		if err != nil {
			b.Fatalf("Error construyendo política: %v", err)
		}

		// 2) Helper para cifrar + serializar
		encrypt := func(plain string) {
			ciph, err := utils.CifrarDatoABE(plain, politica)
			if err != nil {
				b.Fatalf("Error cifrando dato: %v", err)
			}
			if _, err := utils.SerializarCipher(ciph); err != nil {
				b.Fatalf("Error serializando cipher: %v", err)
			}
		}

		// 3) Cifrar todos los campos
		encrypt(ejemplo.Telefono)
		encrypt(ejemplo.Celular)
		encrypt(ejemplo.Direccion)
		encrypt(ejemplo.Ciudad)
		encrypt(ejemplo.Provincia)
		encrypt(ejemplo.FechaNacimiento)
		encrypt(ejemplo.Genero)
		encrypt(ejemplo.EstadoCivil)

		// 4) Upsert real en BD de prueba
		_, err = db.ConnDatos.Exec(context.Background(), `
			INSERT INTO datos_personales 
			  (id_usuario, telefono, celular, direccion, ciudad, provincia,
			   fecha_nacimiento, genero, estado_civil, fecha_creacion)
			VALUES
			  ($1, '', '', '', '', '', '', '', '', NOW())
			ON CONFLICT (id_usuario) DO UPDATE
			  SET fecha_creacion = NOW()
		`, ejemplo.IDUsuario)
		if err != nil {
			b.Fatalf("Error en INSERT/UPSERT: %v", err)
		}
	}
}

// BenchmarkActualizarEscenarioA mide: construir política, cifrar los 8 campos y
// luego hacer un UPDATE en datos_personales.
func BenchmarkActualizarEscenarioA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		politica, err := construirPoliticaDinamica(ejemplo.IDUsuario)
		if err != nil {
			b.Fatalf("Error construyendo política: %v", err)
		}
		encrypt := func(plain string) []byte {
			ciph, err := utils.CifrarDatoABE(plain, politica)
			if err != nil {
				b.Fatalf("Error cifrando dato: %v", err)
			}
			ser, err := utils.SerializarCipher(ciph)
			if err != nil {
				b.Fatalf("Error serializando cipher: %v", err)
			}
			return ser
		}

		// Cifrar todos los campos
		encrypt(ejemplo.Telefono)
		encrypt(ejemplo.Celular)
		encrypt(ejemplo.Direccion)
		encrypt(ejemplo.Ciudad)
		encrypt(ejemplo.Provincia)
		encrypt(ejemplo.FechaNacimiento)
		encrypt(ejemplo.Genero)
		encrypt(ejemplo.EstadoCivil)

		// UPDATE real en BD de prueba
		_, err = db.ConnDatos.Exec(context.Background(), `
			UPDATE datos_personales
			   SET fecha_creacion = NOW()
			 WHERE id_usuario = $1
		`, ejemplo.IDUsuario)
		if err != nil {
			b.Fatalf("Error en UPDATE: %v", err)
		}
	}
}

// BenchmarkEliminarEscenarioA mide la operación DELETE en datos_personales.
func BenchmarkEliminarEscenarioA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := db.ConnDatos.Exec(context.Background(),
			`DELETE FROM datos_personales WHERE id_usuario = $1`, ejemplo.IDUsuario,
		); err != nil {
			b.Fatalf("Error eliminando datos: %v", err)
		}
	}
}
