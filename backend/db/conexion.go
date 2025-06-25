package db

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Declarar variable global del pool
var Pool *pgxpool.Pool

// Conexión a la base de datos de datos_personales (manténla aparte si quieres)
var ConnDatos *pgx.Conn

func ConectarDB() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:postgres@localhost:5432/consentimientos?search_path=public")
	if err != nil {
		log.Fatal("Error creando pool:", err)
	}

	// ASIGNACIÓN AQUÍ:
	Pool = pool

	var exists bool
	err = Pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM usuarios_roles)").Scan(&exists)
	if err != nil {
		log.Fatal("Error en consulta:", err)
	}

	fmt.Println("¿usuarios_roles existe?:", exists)
}

func ConectarDatosPersonales() {
	var err error
	ConnDatos, err = pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5432/datos_personales")
	if err != nil {
		log.Fatalf("Error al conectar con la base de datos (datos_personales): %v", err)
	}
	fmt.Println("Conectado a la base de datos (datos_personales)")
}
