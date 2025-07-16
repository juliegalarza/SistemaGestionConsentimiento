// backend/handlers/dashboard_procesador.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"backend/db"
)

// ObtenerDashboardProcesador maneja GET /procesador/dashboard
func ObtenerDashboardProcesador(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1️⃣ ID del procesador viene desde el middleware
	userIDVal := ctx.Value(CtxUserIDKey)
	idProcesador, ok := userIDVal.(int)
	if !ok {
		http.Error(w, "Usuario no autenticado", http.StatusUnauthorized)
		return
	}

	// 2️⃣ Obtener el nombre
	var nombre string
	if err := db.Pool.QueryRow(ctx, `
		SELECT nombre 
		  FROM usuarios 
		 WHERE id_usuario = $1
	`, idProcesador).Scan(&nombre); err != nil {
		http.Error(w, "Error obteniendo nombre", http.StatusInternalServerError)
		return
	}

	// 3️⃣ Obtener último acceso
	var ultimoAcceso time.Time
	if err := db.Pool.QueryRow(ctx, `
		SELECT ultimo_acceso
		  FROM credenciales_usuarios
		 WHERE id_usuario = $1
	`, idProcesador).Scan(&ultimoAcceso); err != nil {
		// si no hay registro, lo dejamos en cero ("" después al serializar)
	}

	// 4️⃣ Leer los atributos (políticas) asignados al procesador
	var atributos []string
	if err := db.Pool.QueryRow(ctx, `
		SELECT atributos
		  FROM atributos_terceros
		 WHERE id_usuario = $1
	  ORDER BY fecha_asignacion DESC
		 LIMIT 1
	`, idProcesador).Scan(&atributos); err != nil {
		atributos = []string{}
	}

	// 5️⃣ Buscar política que expire en <14 días
	var alertaTitulo string
	var alertaVence time.Time
	if err := db.Pool.QueryRow(ctx, `
		SELECT titulo, fecha_fin
		  FROM politicas_privacidad
		 WHERE fecha_fin > NOW()
		   AND fecha_fin < NOW() + INTERVAL '14 days'
		   AND titulo = ANY($1)
		 ORDER BY fecha_fin ASC
		 LIMIT 1
	`, atributos).Scan(&alertaTitulo, &alertaVence); err != nil {
		alertaTitulo = ""
	}

	// 6️⃣ Contar solicitudes pendientes
	var solicitudes int
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		  FROM solicitudes_atributo
		 WHERE id_procesador = $1
		   AND estado = 'pendiente'
	`, idProcesador).Scan(&solicitudes); err != nil {
		solicitudes = 0
	}

	// 7️⃣ Contar accesos permitidos (consentimientos activos de sus políticas)
	accesos := 0
	for _, attr := range atributos {
		var c int
		if err := db.Pool.QueryRow(ctx, `
			SELECT COUNT(*) 
			  FROM consentimientos c
			  JOIN politicas_privacidad p ON c.id_politica = p.id_politica
			 WHERE p.titulo = $1
			   AND c.estado = 'activo'
			   AND c.fecha_expiracion > NOW()
		`, attr).Scan(&c); err == nil {
			accesos += c
		}
	}

	// 8️⃣ Construir respuesta
	resp := map[string]interface{}{
		"nombre":                 nombre,
		"ultimo_acceso":          ultimoAcceso,
		"solicitudes_pendientes": solicitudes,
		"accesos_permitidos":     accesos,
	}
	if alertaTitulo != "" {
		resp["alerta"] = map[string]interface{}{
			"titulo": alertaTitulo,
			"vence":  alertaVence,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
