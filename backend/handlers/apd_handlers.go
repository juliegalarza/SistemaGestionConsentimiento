// handlers/apd_handlers.go
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"backend/db"

	"github.com/gorilla/mux"
)

// Policy representa la política con conteo de consentimientos activos
type Policy struct {
	ID               int    `json:"id_politica"`
	Titulo           string `json:"titulo"`
	Descripcion      string `json:"descripcion"`
	FechaInicio      string `json:"fecha_inicio"`
	FechaFin         string `json:"fecha_fin"`
	CantidadConsents int    `json:"cantidad_consentimientos"`
}

// AuditPolicy representa una entrada de auditoría sobre políticas
type AuditPolicy struct {
	ID          int    `json:"id"`
	Operacion   string `json:"operacion"`
	PolicyID    int    `json:"id_politica"`
	Titulo      string `json:"titulo"`
	Descripcion string `json:"descripcion"`
	Usuario     string `json:"usuario"`
}

// Consent representa un consentimiento
type Consent struct {
	ID              int    `json:"id_consentimiento"`
	UsuarioID       int    `json:"id_usuario"`
	PolicyID        int    `json:"id_politica"`
	FechaOtorgado   string `json:"fecha_otorgado"`
	FechaExpiracion string `json:"fecha_expiracion"`
	Estado          string `json:"estado"`
}

// AuditEvent representa una entrada de auditoría de eventos
type AuditEvent struct {
	ID          int    `json:"id_evento"`
	UsuarioID   int    `json:"id_usuario"`
	Accion      string `json:"accion"`
	Tabla       string `json:"tabla_afectada"`
	RegistroID  int    `json:"registro_id"`
	Descripcion string `json:"descripcion"`
	FechaEvento string `json:"fecha_evento"`
}

// ListPolicies GET /apd/api/policies
// Lista todas las políticas junto al conteo de consentimientos activos
func ListPolicies(w http.ResponseWriter, r *http.Request) {
	const sql = `
SELECT p.id_politica, p.titulo, p.descripcion,
       p.fecha_inicio, p.fecha_fin,
       COUNT(c.id_consentimiento) AS cantidad_consentimientos
  FROM politicas_privacidad p
  LEFT JOIN consentimientos c
    ON c.id_politica = p.id_politica
   AND c.estado = 'activo'
 GROUP BY p.id_politica
 ORDER BY p.id_politica`

	rows, err := db.Pool.Query(context.Background(), sql)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []Policy
	for rows.Next() {
		var p Policy
		var fi, ff time.Time
		if err := rows.Scan(
			&p.ID, &p.Titulo, &p.Descripcion,
			&fi, &ff,
			&p.CantidadConsents,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		p.FechaInicio = fi.Format(time.RFC3339)
		p.FechaFin = ff.Format(time.RFC3339)
		list = append(list, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// PolicyHistory GET /apd/api/policies/{id}/history
// Devuelve el historial de auditoría de la política dada
func PolicyHistory(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	const sqlQuery = `
SELECT id, operacion, id_politica, titulo, descripcion, usuario
  FROM auditoria_politicas
 WHERE id_politica = $1
 ORDER BY id DESC`

	rows, err := db.Pool.Query(context.Background(), sqlQuery, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hist []AuditPolicy
	for rows.Next() {
		var a AuditPolicy
		var usr sql.NullString

		if err := rows.Scan(
			&a.ID,
			&a.Operacion,
			&a.PolicyID,
			&a.Titulo,
			&a.Descripcion,
			&usr,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Si usuario es NULL, devolvemos cadena vacía
		if usr.Valid {
			a.Usuario = usr.String
		} else {
			a.Usuario = ""
		}

		hist = append(hist, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hist)
}

// ListConsents GET /apd/api/consents
// Lista todos los consentimientos registrados
func ListConsents(w http.ResponseWriter, r *http.Request) {
	const sql = `
SELECT id_consentimiento, id_usuario, id_politica,
       fecha_otorgado, fecha_expiracion, estado
  FROM consentimientos
 ORDER BY fecha_otorgado DESC`

	rows, err := db.Pool.Query(context.Background(), sql)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []Consent
	for rows.Next() {
		var c Consent
		var fo time.Time
		var exp *time.Time

		if err := rows.Scan(
			&c.ID, &c.UsuarioID, &c.PolicyID,
			&fo, &exp, &c.Estado,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// formateamos fecha otorgado
		c.FechaOtorgado = fo.Format(time.RFC3339)
		// si exp es nil, dejamos cadena vacía
		if exp != nil {
			c.FechaExpiracion = exp.Format(time.RFC3339)
		} else {
			c.FechaExpiracion = ""
		}

		list = append(list, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// ConsentHistory GET /apd/api/consents/{id}/history
// Devuelve el historial de auditoría de un consentimiento concreto
func ConsentHistory(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	const sql = `
SELECT id_evento, id_usuario, accion,
       tabla_afectada, registro_id,
       descripcion, fecha_evento
  FROM auditoria_eventos
 WHERE tabla_afectada = 'consentimientos'
   AND registro_id = $1
 ORDER BY fecha_evento DESC`

	rows, err := db.Pool.Query(context.Background(), sql, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hist []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var fe time.Time
		if err := rows.Scan(
			&e.ID, &e.UsuarioID, &e.Accion,
			&e.Tabla, &e.RegistroID,
			&e.Descripcion, &fe,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		e.FechaEvento = fe.Format(time.RFC3339)
		hist = append(hist, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hist)
}
