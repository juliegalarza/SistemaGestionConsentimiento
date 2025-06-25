package handlers

import (
	"backend/db"
	"encoding/json"
	"log"
	"net/http"
)

// Vista plana para devolver por JSON al custodio
type ConsentimientoView struct {
	IDConsentimiento int    `json:"id_consentimiento"`
	Usuario          string `json:"usuario"`
	Politica         string `json:"politica"`
	FechaOtorgado    string `json:"fecha_otorgado"`
	FechaExpiracion  string `json:"fecha_expiracion"`
	Estado           string `json:"estado"`
}

// GET /custodio/consentimientos
func ObtenerConsentimientosCustodio(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
		SELECT
			id_consentimiento,
			usuario, -- viene de u.nombre AS usuario
			politica, -- viene de p.titulo AS politica
			coalesce(
			  to_char(fecha_otorgado,  'YYYY-MM-DD"T"HH24:MI:SSZ'),
			  ''
			) AS fecha_otorgado,
			coalesce(
			  to_char(fecha_expiracion, 'YYYY-MM-DD'),
			  ''
			) AS fecha_expiracion,
			estado
		FROM vw_consentimientos_custodio
		ORDER BY fecha_otorgado DESC
	`)
	if err != nil {
		log.Printf("Error en Query vw_consentimientos_custodio: %v", err)
		http.Error(w, "Error leyendo consentimientos custodia", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ConsentimientoView struct {
		IDConsentimiento int    `json:"id_consentimiento"`
		Usuario          string `json:"usuario"`
		Politica         string `json:"politica"`
		FechaOtorgado    string `json:"fecha_otorgado"`
		FechaExpiracion  string `json:"fecha_expiracion"`
		Estado           string `json:"estado"`
	}

	var lista []ConsentimientoView
	for rows.Next() {
		var c ConsentimientoView
		if err := rows.Scan(
			&c.IDConsentimiento,
			&c.Usuario,
			&c.Politica,
			&c.FechaOtorgado,
			&c.FechaExpiracion,
			&c.Estado,
		); err != nil {
			log.Printf("scan custodia consentimientos: %v", err)
			continue
		}
		lista = append(lista, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}
