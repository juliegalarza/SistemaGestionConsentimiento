// backend/handlers/monitoreo.go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/db"
	"backend/models"
)

func MonitorConsentimientos(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(), `
        SELECT
          ae.id_evento,
          u.nombre                  AS nombre_usuario,
          COALESCE(rp.nombre,'Sin rol') AS rol_usuario,
          p.titulo                  AS accion,
          ae.fecha_evento,
          CASE
            WHEN ae.tabla_afectada='consentimientos'
                 AND c.estado='activo' THEN 'Concedido'
            WHEN ae.tabla_afectada='consentimientos'
                 AND c.estado IN ('expirado','revocado','no_aceptado') THEN 'Denegado'
            ELSE 'Desconocido'
          END                        AS permiso_estado,
          p.titulo                   AS politica_nombre,
          c.fecha_otorgado           AS fecha_consentimiento,
          c.fecha_expiracion         AS fecha_expiracion_consent,
          ae.descripcion,
          ut.nombre                  AS titular_nombre
        FROM auditoria_eventos ae
        JOIN usuarios u 
          ON u.id_usuario = ae.id_usuario
        LEFT JOIN (
          SELECT ur.id_usuario, r.nombre
            FROM usuarios_roles ur
            JOIN roles r 
              ON r.id_rol = ur.id_rol
           WHERE (ur.id_usuario, ur.fecha_asignacion) IN (
             SELECT id_usuario, MAX(fecha_asignacion)
               FROM usuarios_roles
              GROUP BY id_usuario
           )
        ) rp ON rp.id_usuario = ae.id_usuario
        LEFT JOIN consentimientos c
          ON ae.tabla_afectada='consentimientos'
         AND c.id_consentimiento = ae.registro_id
        LEFT JOIN politicas_privacidad p
          ON p.id_politica = c.id_politica
        LEFT JOIN usuarios ut
          ON ut.id_usuario = c.id_usuario   -- el titular del consentimiento
        ORDER BY ae.fecha_evento DESC
    `)
	if err != nil {
		http.Error(w, "Error al leer auditoría: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []models.EventoMonitoreo
	for rows.Next() {
		var ev models.EventoMonitoreo
		if err := rows.Scan(
			&ev.IDEvento,
			&ev.IDUsuario,
			&ev.Accion,
			&ev.TablaAfectada,
			&ev.RegistroID,
			&ev.Descripcion,
			&ev.FechaEvento,
		); err != nil {
			http.Error(w, "Error mapeando fila: "+err.Error(), http.StatusInternalServerError)
			return
		}
		lista = append(lista, ev)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}
