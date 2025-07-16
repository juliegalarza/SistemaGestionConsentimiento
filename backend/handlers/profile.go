// handlers/profile.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/db"
)

type Profile struct {
	Nombre       string    `json:"nombre"`
	Rol          string    `json:"rol"`
	UltimoAcceso time.Time `json:"ultimo_acceso"`
}

func ObtenerPerfilCustodio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1) Leer X-User-ID
	idStr := r.Header.Get("X-User-ID")
	if idStr == "" {
		http.Error(w, "X-User-ID requerido", http.StatusBadRequest)
		return
	}
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "X-User-ID inválido", http.StatusBadRequest)
		return
	}

	// 2) Query: nombre, rol y último login exitoso
	var p Profile
	err = db.Pool.QueryRow(ctx, `
    SELECT
      u.nombre,
      -- Subconsulta para el último login exitoso
      (SELECT MAX(fecha_intento)
         FROM auditoria_login
        WHERE id_usuario = u.id_usuario
          AND exito = true
      )        AS ultimo_acceso,
      r.nombre AS rol
    FROM usuarios AS u

    -- último rol asignado
    JOIN usuarios_roles AS ur
      ON ur.id_usuario = u.id_usuario
     AND ur.fecha_asignacion = (
       SELECT MAX(fecha_asignacion)
         FROM usuarios_roles
        WHERE id_usuario = u.id_usuario
     )
    JOIN roles AS r
      ON r.id_rol = ur.id_rol

    WHERE u.id_usuario = $1
  `, userID).Scan(&p.Nombre, &p.UltimoAcceso, &p.Rol)
	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}
