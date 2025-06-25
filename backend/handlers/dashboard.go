// handlers/dashboard.go

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/db"
)

func Dashboard(w http.ResponseWriter, r *http.Request) {
	// 1) Leer id_usuario
	usr := r.URL.Query().Get("id_usuario")
	if usr == "" {
		http.Error(w, "Falta id_usuario", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(usr)
	if err != nil {
		http.Error(w, "id_usuario inválido", http.StatusBadRequest)
		return
	}

	// 2) Obtener nombre y último acceso
	var nombre string
	var ultimoAcceso time.Time
	err = db.Pool.QueryRow(context.Background(), `
		SELECT u.nombre, cu.ultimo_acceso
		  FROM usuarios u
		  JOIN credenciales_usuarios cu ON cu.id_usuario = u.id_usuario
		 WHERE u.id_usuario = $1
	`, id).Scan(&nombre, &ultimoAcceso)
	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	// 3) Contar totales
	var total, activos, expirados int
	err = db.Pool.QueryRow(context.Background(), `
		SELECT 
		  count(*) AS total,
		  count(*) FILTER (WHERE estado = 'activo')   AS activos,
		  count(*) FILTER (WHERE estado = 'expirado') AS expirados
		FROM consentimientos
		WHERE id_usuario = $1
	`, id).Scan(&total, &activos, &expirados)
	if err != nil {
		http.Error(w, "Error al calcular resumen", http.StatusInternalServerError)
		return
	}

	// 4) Traer últimos 4 consentimientos (ordenados por fecha_expiracion asc para próximos a expirar)
	rows, err := db.Pool.Query(context.Background(), `
		SELECT id_consentimiento, id_politica, fecha_otorgado, fecha_expiracion, estado
		  FROM consentimientos
		 WHERE id_usuario = $1
		   AND estado = 'activo'
		 ORDER BY fecha_expiracion ASC
		 LIMIT 4
	`, id)
	if err != nil {
		http.Error(w, "Error al cargar consentimientos", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Estructura con id_politica
	type Consent struct {
		ID              int        `json:"id"`
		IDPolitica      int        `json:"id_politica"`
		FechaOtorgado   time.Time  `json:"fecha_otorgado"`
		FechaExpiracion *time.Time `json:"fecha_expiracion"`
		Estado          string     `json:"estado"`
	}

	var consents []Consent
	for rows.Next() {
		var c Consent
		if err := rows.Scan(&c.ID, &c.IDPolitica, &c.FechaOtorgado, &c.FechaExpiracion, &c.Estado); err != nil {
			http.Error(w, "Error al leer consentimientos", http.StatusInternalServerError)
			return
		}
		consents = append(consents, c)
	}

	// 5) Construir y enviar JSON
	resp := map[string]interface{}{
		"nombre":        nombre,
		"ultimo_acceso": ultimoAcceso,
		"summary": map[string]int{
			"total_consentimientos": total,
			"activos":               activos,
			"expirados":             expirados,
		},
		"consentimientos": consents,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
