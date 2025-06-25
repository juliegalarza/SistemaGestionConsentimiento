package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"backend/db"
)

type PoliticaPrivacidad struct {
	IDPolitica  int       `json:"id_politica"`
	Titulo      string    `json:"titulo"`
	Descripcion string    `json:"descripcion"`
	FechaInicio time.Time `json:"fecha_inicio"`
	FechaFin    time.Time `json:"fecha_fin"`
	Estado      string    `json:"estado"`
}

/*func ObtenerPoliticasParaProcesador(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(CtxUserIDKey).(int)

	// Paso 1: leer el campo atributos como jsonb (string) y luego decodificar
	var atributosJSON string
	err := db.Pool.QueryRow(r.Context(), `
		SELECT atributos FROM atributos_terceros WHERE id_usuario = $1
	`, userID).Scan(&atributosJSON)
	if err != nil {
		http.Error(w, "Error cargando atributos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var atributos []string
	err = json.Unmarshal([]byte(atributosJSON), &atributos)
	if err != nil {
		http.Error(w, "Error parseando atributos JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Paso 2: buscar políticas cuyo título esté entre esos atributos
	rows, err := db.Pool.Query(r.Context(), `
		SELECT id_politica, titulo, descripcion, fecha_inicio, fecha_fin
		FROM politicas_privacidad
		WHERE titulo = ANY($1)
	`, atributos)
	if err != nil {
		http.Error(w, "Error consultando políticas: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var politicas []PoliticaPrivacidad
	for rows.Next() {
		var p PoliticaPrivacidad
		err := rows.Scan(&p.IDPolitica, &p.Titulo, &p.Descripcion, &p.FechaInicio, &p.FechaFin)
		if err != nil {
			http.Error(w, "Error al procesar resultados: "+err.Error(), http.StatusInternalServerError)
			return
		}
		p.Estado = "Concedido"
		politicas = append(politicas, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(politicas)
} */

func ObtenerPoliticasParaProcesador(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(CtxUserIDKey).(int)

	// Obtener políticas concedidas por coincidencia con sus atributos
	var atributosJSON string
	err := db.Pool.QueryRow(r.Context(), `
		SELECT atributos FROM atributos_terceros WHERE id_usuario = $1
	`, userID).Scan(&atributosJSON)
	if err != nil {
		http.Error(w, "Error cargando atributos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var atributos []string
	if err := json.Unmarshal([]byte(atributosJSON), &atributos); err != nil {
		http.Error(w, "Error parseando JSON de atributos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows1, err := db.Pool.Query(r.Context(), `
		SELECT id_politica, titulo, descripcion, fecha_inicio, fecha_fin
		FROM politicas_privacidad
		WHERE titulo = ANY($1)
	`, atributos)
	if err != nil {
		http.Error(w, "Error consultando políticas concedidas", http.StatusInternalServerError)
		return
	}
	defer rows1.Close()

	type Politica struct {
		IDPolitica  int       `json:"id_politica"`
		Titulo      string    `json:"titulo"`
		Descripcion string    `json:"descripcion"`
		FechaInicio time.Time `json:"fecha_inicio"`
		FechaFin    time.Time `json:"fecha_fin"`
		Estado      string    `json:"estado"`
	}

	var politicas []Politica

	for rows1.Next() {
		var p Politica
		if err := rows1.Scan(&p.IDPolitica, &p.Titulo, &p.Descripcion, &p.FechaInicio, &p.FechaFin); err != nil {
			http.Error(w, "Error procesando políticas concedidas", http.StatusInternalServerError)
			return
		}
		p.Estado = "Concedido"
		politicas = append(politicas, p)
	}

	// Agregar solicitudes del procesador (pendiente, aprobado, denegado)
	rows2, err := db.Pool.Query(r.Context(), `
		SELECT p.id_politica, p.titulo, p.descripcion, p.fecha_inicio, p.fecha_fin, s.estado
		FROM solicitudes_atributo s
		JOIN politicas_privacidad p ON s.id_politica = p.id_politica
		WHERE s.id_procesador = $1
	`, userID)
	if err != nil {
		http.Error(w, "Error consultando solicitudes", http.StatusInternalServerError)
		return
	}
	defer rows2.Close()

	for rows2.Next() {
		var p Politica
		if err := rows2.Scan(&p.IDPolitica, &p.Titulo, &p.Descripcion, &p.FechaInicio, &p.FechaFin, &p.Estado); err != nil {
			http.Error(w, "Error procesando solicitudes", http.StatusInternalServerError)
			return
		}
		politicas = append(politicas, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(politicas)
}

func ObtenerTodasLasPoliticas(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
		SELECT id_politica, titulo, descripcion, fecha_inicio, fecha_fin
		FROM politicas_privacidad
	`)
	if err != nil {
		http.Error(w, "Error cargando políticas", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var politicas []PoliticaPrivacidad
	for rows.Next() {
		var p PoliticaPrivacidad
		if err := rows.Scan(&p.IDPolitica, &p.Titulo, &p.Descripcion, &p.FechaInicio, &p.FechaFin); err != nil {
			http.Error(w, "Error procesando resultados", http.StatusInternalServerError)
			return
		}
		politicas = append(politicas, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(politicas)
}
