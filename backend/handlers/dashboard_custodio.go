package handlers

import (
	"encoding/json"
	"net/http"

	"backend/db"
)

type RazonesItem struct {
	Motivo string `json:"motivo"`
	Count  int    `json:"count"`
}

type DashboardResponse struct {
	AccesosRecientes int           `json:"accesosRecientes"`
	Fallos           int           `json:"fallos"`
	Razones          []RazonesItem `json:"razones"`
}

// ObtenerDashboard devuelve las métricas necesarias para el Dashboard Custodio.
func ObtenerDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1) Total accesos recientes (p.ej últimos 7 días; ajusta tu filtro si quieres otro rango)
	var accesos int
	err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		  FROM auditoria_eventos 
		 WHERE tabla_afectada='consentimientos'
		   AND fecha_evento > NOW() - INTERVAL '7 days'
	`).Scan(&accesos)
	if err != nil {
		http.Error(w, "Error contando accesos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2) Total fallos
	var fallos int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		  FROM auditoria_eventos 
		 WHERE tabla_afectada='consentimientos' 
		   AND accion LIKE 'FALLO%'
	`).Scan(&fallos)
	if err != nil {
		http.Error(w, "Error contando fallos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3) Conteo de motivos para el chart
	rows, err := db.Pool.Query(ctx, `
		SELECT 
		  CASE 
		    WHEN error_mensaje ILIKE '%atributo no valido%' THEN 'Atributo no válido'
		    WHEN error_mensaje ILIKE '%revocada%'        THEN 'Política revocada'
		    WHEN error_mensaje ILIKE '%expira%'          THEN 'Atributo expirado'
		    ELSE 'Otro' 
		  END AS motivo,
		  COUNT(*) 
		FROM auditoria_eventos
		WHERE tabla_afectada='consentimientos' AND accion LIKE 'FALLO%'
		GROUP BY motivo
	`)
	if err != nil {
		http.Error(w, "Error agrupando motivos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var razones []RazonesItem
	for rows.Next() {
		var it RazonesItem
		if err := rows.Scan(&it.Motivo, &it.Count); err != nil {
			continue
		}
		razones = append(razones, it)
	}

	resp := DashboardResponse{
		AccesosRecientes: accesos,
		Fallos:           fallos,
		Razones:          razones,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
