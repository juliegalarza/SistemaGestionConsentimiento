package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"backend/db"
)

// Fallo refleja un registro de auditoría de tipo “FALLO” en consentimientos.
type Fallo struct {
	IDEvento     int       `json:"id_evento"`
	FechaEvento  time.Time `json:"fecha_evento"`
	UsuarioID    int       `json:"usuario_id"`
	Accion       string    `json:"accion"`
	Tabla        string    `json:"tabla_afectada"`
	RegistroID   *int      `json:"registro_id,omitempty"`
	Descripcion  string    `json:"descripcion"`
	Exito        bool      `json:"exito"`
	ErrorMensaje *string   `json:"error_mensaje,omitempty"`
}

// FallosResponse agrupa las métricas y los últimos fallos.
type FallosResponse struct {
	Total      int     `json:"total"`
	Fallidos   int     `json:"fallidos"`
	Porcentaje int     `json:"porcentaje"` // % redondeado
	Recientes  []Fallo `json:"recientes"`
}

func ObtenerFallos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1) Contar totales y fallidos
	var total, fallidos int
	err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE tabla_afectada='consentimientos')                                       AS total,
			COUNT(*) FILTER (WHERE tabla_afectada='consentimientos' AND accion LIKE 'FALLO%')                AS fallidos
		  FROM auditoria_eventos
	`).Scan(&total, &fallidos)
	if err != nil {
		http.Error(w, "Error contando eventos de auditoría: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2) Calcular porcentaje
	porc := 0
	if total > 0 {
		porc = int(float64(fallidos) / float64(total) * 100.0)
	}

	// 3) Leer los últimos 10 fallos (incluyendo exito y error_mensaje)
	rows, err := db.Pool.Query(ctx, `
		SELECT
			id_evento,
			fecha_evento,
			id_usuario,
			accion,
			tabla_afectada,
			registro_id,
			descripcion,
			exito,
			error_mensaje
		  FROM auditoria_eventos
		 WHERE tabla_afectada='consentimientos'
		   AND accion LIKE 'FALLO%' 
		 ORDER BY fecha_evento DESC
		 LIMIT 10
	`)
	if err != nil {
		http.Error(w, "Error obteniendo registros de fallos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var recientes []Fallo
	for rows.Next() {
		var f Fallo
		if err := rows.Scan(
			&f.IDEvento,
			&f.FechaEvento,
			&f.UsuarioID,
			&f.Accion,
			&f.Tabla,
			&f.RegistroID,
			&f.Descripcion,
			&f.Exito,
			&f.ErrorMensaje,
		); err != nil {
			continue
		}
		recientes = append(recientes, f)
	}

	// 4) Devolver JSON
	resp := FallosResponse{
		Total:      total,
		Fallidos:   fallidos,
		Porcentaje: porc,
		Recientes:  recientes,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
