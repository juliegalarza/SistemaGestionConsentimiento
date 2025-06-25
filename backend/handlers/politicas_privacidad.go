// backend/handlers/politicas_privacidad.go
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/db"
	"backend/models"

	"github.com/gorilla/mux"
)

// --------------------------------------
//  Modelo esperado
// --------------------------------------
// En backend/models/politicas_privacidad.go deberías tener algo así:
//
// package models
//
// import "time"
//
// type PoliticaPrivacidad struct {
//     ID          int       `json:"id_politica"`
//     Titulo      string    `json:"titulo"`
//     Descripcion string    `json:"descripcion"`
//     FechaInicio time.Time `json:"fecha_inicio"`
//     FechaFin    time.Time `json:"fecha_fin"`
// }
//

// --------------------------------------
//  Handler: ObtenerPoliticas (GET /politicas?id_usuario=<X>)
// --------------------------------------
//
// Devuelve todas las políticas registradas, e indica para cada una
// el estado respecto al usuario pasado como parámetro:
//   - "Pendiente"    → el usuario nunca la aceptó ni la rechazó.
//   - "Aceptado"     → existe un consentimiento activo (estado='activo' y fecha_expiracion>NOW()).
//   - "No aceptado"  → existe un consentimiento con estado='no_aceptado' (rechazado).
//   - "Revocado"     → existe un consentimiento con estado='revocado'.
//   - "Expirado"     → existe un consentimiento con estado='expirado'.
//
// Si no se envía id_usuario o es inválido, devuelve 400.

/*func ObtenerPoliticas(w http.ResponseWriter, r *http.Request) {
	// 1) Leer id_usuario del query param
	usr := r.URL.Query().Get("id_usuario")
	idUsuario, err := strconv.Atoi(usr)
	if err != nil {
		http.Error(w, "id_usuario inválido", http.StatusBadRequest)
		return
	}

	// 2) Realizar el LEFT JOIN entre politicas_privacidad y consentimientos de ese usuario
	//    para obtener el estado (si existe) y la fecha_expiracion correspondiente.
	rows, err := db.Conn.Query(context.Background(), `
	  SELECT
		p.id_politica,
		p.titulo,
		p.descripcion,
		p.fecha_inicio,
		p.fecha_fin,
		c.estado           AS estado_cons,
		c.fecha_expiracion AS exp_user
	  FROM politicas_privacidad p
	  LEFT JOIN consentimientos c
		ON p.id_politica = c.id_politica
	   AND c.id_usuario   = $1
	`, idUsuario)
	if err != nil {
		http.Error(w, "Error al obtener políticas", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// 3) Para devolver JSON con el estado interpretado
	type PoliticaConEstado struct {
		ID          int       `json:"id_politica"`
		Titulo      string    `json:"titulo"`
		Descripcion string    `json:"descripcion"`
		FechaInicio time.Time `json:"fecha_inicio"`
		FechaFin    time.Time `json:"fecha_fin"`
		Estado      string    `json:"estado"`
	}

	var lista []PoliticaConEstado

	for rows.Next() {
		var p PoliticaConEstado
		var estadoCons sql.NullString
		var expUser *time.Time

		if err := rows.Scan(
			&p.ID,
			&p.Titulo,
			&p.Descripcion,
			&p.FechaInicio,
			&p.FechaFin,
			&estadoCons,
			&expUser,
		); err != nil {
			continue
		}

		// 4) Interpretar el estado según los valores en consentimientos
		switch {
		case !estadoCons.Valid:
			// No hay ningún consentimiento para esta política y usuario
			p.Estado = "Pendiente"

		case estadoCons.String == "no_aceptado":
			p.Estado = "No aceptado"

		case estadoCons.String == "revocado":
			p.Estado = "Revocado"

		case estadoCons.String == "expirado":
			p.Estado = "Expirado"

		default:
			// Si existe estadoCons y no es ninguno de los anteriores,
			// asumimos que es "activo" (u otro estado que quieras mapear).
			p.Estado = "Aceptado"
		}

		lista = append(lista, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}*/

func TituloDePolitica(idPol int) string {
	var titulo string
	err := db.Pool.QueryRow(context.Background(),
		"SELECT titulo FROM politicas_privacidad WHERE id_politica = $1",
		idPol,
	).Scan(&titulo)
	if err != nil {
		log.Printf("Error al obtener título de política %d: %v", idPol, err)
		return "(desconocida)"
	}
	return titulo
}

// PoliticaConEstado es el JSON que devolvemos al frontend
type PoliticaConEstado struct {
	ID          int       `json:"id_politica"`
	Titulo      string    `json:"titulo"`
	Descripcion string    `json:"descripcion"`
	FechaInicio time.Time `json:"fecha_inicio"`
	FechaFin    time.Time `json:"fecha_fin"`
	Estado      string    `json:"estado"` // “Pendiente”, “Aceptado”, “Expirado”, etc.
}

// GET /politicas?id_usuario=...
func ObtenerPoliticas(w http.ResponseWriter, r *http.Request) {
	// 1) Leer id_usuario del query string
	usr := r.URL.Query().Get("id_usuario")
	idUsuario, err := strconv.Atoi(usr)
	if err != nil {
		http.Error(w, "id_usuario inválido", http.StatusBadRequest)
		return
	}

	// 2) Sub‐SELECT para traer sólo el consentimiento más reciente por política
	rows, err := db.Pool.Query(r.Context(), `
        SELECT
            p.id_politica,
            p.titulo,
            p.descripcion,
            p.fecha_inicio,
            p.fecha_fin,
            c.estado           AS estado_cons,
            c.fecha_expiracion AS exp_user
        FROM politicas_privacidad p
        LEFT JOIN (
            SELECT DISTINCT ON (id_politica)
                   id_politica,
                   estado,
                   fecha_expiracion
              FROM consentimientos
             WHERE id_usuario = $1
             ORDER BY id_politica, fecha_otorgado DESC
        ) c
          ON p.id_politica = c.id_politica
    `, idUsuario)

	if err != nil {
		http.Error(w, "Error al obtener políticas: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []PoliticaConEstado
	for rows.Next() {
		var p PoliticaConEstado
		var estadoCons sql.NullString
		var expUser *time.Time

		if err := rows.Scan(
			&p.ID, &p.Titulo, &p.Descripcion, &p.FechaInicio, &p.FechaFin,
			&estadoCons, &expUser,
		); err != nil {
			continue
		}

		// 3) Determinar el texto de “Estado” que mandaremos al frontend
		switch {
		case !estadoCons.Valid:
			// No hay ningún consentimiento previo
			p.Estado = "Pendiente"
		case estadoCons.String == "no_aceptado":
			p.Estado = "No aceptado"
		case estadoCons.String == "revocado":
			p.Estado = "Revocado"
		case estadoCons.String == "expirado":
			p.Estado = "Expirado"
		default:
			// Cualquier otro caso lo consideramos “Activo” → mostrar “Aceptado”
			p.Estado = "Aceptado"
		}

		lista = append(lista, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}

// --------------------------------------
//  Handler: ObtenerPoliticaPorID (GET /politicas/{id})
// --------------------------------------
//
// Devuelve una sola política, buscando por id_politica en path param.
// Si no existe, retorna 404.

func ObtenerPoliticaPorID(w http.ResponseWriter, r *http.Request) {
	// Leer "id" del path: /politicas/{id}
	vars := mux.Vars(r)
	idStr := vars["id"]
	if idStr == "" {
		http.Error(w, "Falta ID de política", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var p models.PoliticaPrivacidad
	err = db.Pool.QueryRow(context.Background(),
		`SELECT id_politica, titulo, descripcion, fecha_inicio, fecha_fin
         FROM politicas_privacidad
         WHERE id_politica = $1`, id).
		Scan(&p.ID, &p.Titulo, &p.Descripcion, &p.FechaInicio, &p.FechaFin)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Política no encontrada", http.StatusNotFound)
		} else {
			http.Error(w, "Error al leer política: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// --------------------------------------
//  Handler: CrearPolitica (POST /politicas)
// --------------------------------------
//
// Recibe JSON con campos {titulo, descripcion, fecha_inicio, fecha_fin}.
// Inserta una nueva fila en politicas_privacidad. Retorna 201 y mensaje de éxito.

func CrearPolitica(w http.ResponseWriter, r *http.Request) {
	var input models.PoliticaPrivacidad
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	// Validación mínima: título no vacío, fechas coherentes, etc.
	if strings.TrimSpace(input.Titulo) == "" {
		http.Error(w, "El título no puede quedar vacío", http.StatusBadRequest)
		return
	}
	if input.FechaFin.Before(input.FechaInicio) {
		http.Error(w, "La fecha de fin no puede ser anterior a la fecha de inicio", http.StatusBadRequest)
		return
	}

	_, err := db.Pool.Exec(context.Background(), `
		INSERT INTO politicas_privacidad (titulo, descripcion, fecha_inicio, fecha_fin)
		VALUES ($1, $2, $3, $4)
	`, input.Titulo, input.Descripcion, input.FechaInicio, input.FechaFin)

	if err != nil {
		http.Error(w, "Error al guardar política: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Política creada correctamente"})
}

// --------------------------------------
//  Handler: ActualizarPolitica (PUT /politicas)
// --------------------------------------
//
// Recibe JSON con {id_politica, titulo, descripcion, fecha_inicio, fecha_fin}.
// Actualiza los datos de la política existente.

func ActualizarPolitica(w http.ResponseWriter, r *http.Request) {
	var input models.PoliticaPrivacidad
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	// Validar que exista primero (opcional, puedes directamente hacer UPDATE)
	var existe bool
	err := db.Pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM politicas_privacidad WHERE id_politica=$1)",
		input.ID).Scan(&existe)
	if err != nil {
		http.Error(w, "Error verificando existencia de política: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !existe {
		http.Error(w, "La política no existe", http.StatusNotFound)
		return
	}

	// Validación mínima de fechas
	if input.FechaFin.Before(input.FechaInicio) {
		http.Error(w, "La fecha de fin no puede ser anterior a la fecha de inicio", http.StatusBadRequest)
		return
	}

	_, err = db.Pool.Exec(context.Background(), `
		UPDATE politicas_privacidad
		SET titulo = $1, descripcion = $2, fecha_inicio = $3, fecha_fin = $4
		WHERE id_politica = $5
	`, input.Titulo, input.Descripcion, input.FechaInicio, input.FechaFin, input.ID)

	if err != nil {
		http.Error(w, "Error al actualizar política: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Política actualizada correctamente"})
}

// --------------------------------------
//  Handler: EliminarPolitica (DELETE /politicas?id_politica=<X>)
// --------------------------------------
//
// Borra la política indicada por query param “id_politica”.

func EliminarPolitica(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id_politica")
	if idStr == "" {
		http.Error(w, "Falta id_politica", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "id_politica inválido", http.StatusBadRequest)
		return
	}

	// Verificar si existe (opcional)
	var existe bool
	err = db.Pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM politicas_privacidad WHERE id_politica=$1)", id).
		Scan(&existe)
	if err != nil {
		http.Error(w, "Error verificando existencia de política: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !existe {
		http.Error(w, "La política no existe", http.StatusNotFound)
		return
	}

	// Eliminar
	_, err = db.Pool.Exec(context.Background(),
		"DELETE FROM politicas_privacidad WHERE id_politica = $1", id)
	if err != nil {
		http.Error(w, "Error al eliminar política: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Política eliminada correctamente"})
}
