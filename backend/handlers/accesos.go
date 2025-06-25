// backend/handlers/accesos.go
package handlers

import (
	"backend/db"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Registro plano para devolver por JSON
type AccesoView struct {
	IDAcceso         int       `json:"id_acceso"`
	UsuarioNombre    string    `json:"usuario"`
	Rol              string    `json:"rol"`
	Atributo         string    `json:"atributo"`
	IDConsentimiento int       `json:"consentimiento"`
	FechaOtorgado    string    `json:"fecha_otorgado,omitempty"`
	FechaExpiracion  string    `json:"fecha_expiracion,omitempty"`
	EstadoConsent    string    `json:"estado,omitempty"`
	FechaEvento      time.Time `json:"fecha_evento"`
	Resultado        string    `json:"resultado"`
	MotivoFallo      string    `json:"motivo_fallo"`
}

// GET /custodio/accesos
// GET /custodio/accesos
func ObtenerAccesosCustodio(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
      SELECT 
        id_acceso,
      usuario,
      rol,
      atributo,
      id_consentimiento,
      fecha_evento,
      resultado,
      motivo_fallo
    FROM vw_accesos_custodio
    ORDER BY fecha_evento DESC
    `)
	if err != nil {
		log.Printf("Error en Query vw_accesos_custodio: %v", err)
		http.Error(w, "Error leyendo accesos custodia", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []AccesoView
	for rows.Next() {
		var a AccesoView
		if err := rows.Scan(
			&a.IDAcceso,
			&a.UsuarioNombre,
			&a.Rol,
			&a.Atributo,
			&a.IDConsentimiento,
			&a.FechaEvento,
			&a.Resultado,
			&a.MotivoFallo,
		); err != nil {
			log.Println("scan custodia:", err)
			continue
		}
		lista = append(lista, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}

// GET /auditor/accesos
func ObtenerAccesosAuditor(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
      SELECT 
        id_acceso,
        usuario_nombre,
        rol,
        atributo,
        id_consentimiento,
        fecha_otorgado,
        fecha_expiracion,
        estado,
        fecha_evento,
        CASE WHEN exito THEN 'Autorizado' ELSE 'Denegado' END as resultado,
        motivo as motivo_fallo
      FROM vw_accesos_auditor
    `)
	if err != nil {
		http.Error(w, "Error leyendo accesos auditoría", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []AccesoView
	for rows.Next() {
		var a AccesoView
		if err := rows.Scan(
			&a.IDAcceso,
			&a.UsuarioNombre,
			&a.Rol,
			&a.Atributo,
			&a.IDConsentimiento,
			&a.FechaOtorgado,
			&a.FechaExpiracion,
			&a.EstadoConsent,
			&a.FechaEvento,
			&a.Resultado,
			&a.MotivoFallo,
		); err != nil {
			log.Println("scan auditor:", err)
			continue
		}
		lista = append(lista, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}

// LogAcceso registra cada intento de descifrado, usando ahora el consent ID.
func LogAcceso(ctx context.Context, userID, consentID int, exito bool, motivo string) {
	_, err := db.Pool.Exec(ctx, `
    INSERT INTO accesos (
      id_solicitante,
      id_consentimiento,
      exito,
      motivo,
      fecha_evento
    ) VALUES ($1, $2, $3, $4, NOW())
  `, userID, consentID, exito, motivo)
	if err != nil {
		log.Printf("Error registrando acceso (user=%d, consent=%d): %v", userID, consentID, err)
	}
}
