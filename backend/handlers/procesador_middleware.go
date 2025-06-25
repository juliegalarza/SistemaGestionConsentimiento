// backend/handlers/procesador_middleware.go
package handlers

import (
	"context"
	"net/http"
	"strconv"

	"backend/db"
)

// backend/handlers/procesador_middleware.go
func ProcesadorOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.Header.Get("X-User-ID")
		if idStr == "" {
			http.Error(w, "No autenticado", http.StatusUnauthorized)
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "ID inválido", http.StatusBadRequest)
			return
		}
		var exists int
		err = db.Pool.QueryRow(r.Context(), `
			SELECT 1 FROM usuarios_roles
			WHERE id_usuario=$1 AND id_rol=3
			LIMIT 1
		`, id).Scan(&exists)
		if err != nil {
			http.Error(w, "Acceso denegado: se requiere rol Procesador", http.StatusForbidden)
			return
		}

		// ✅ Agregar correctamente al contexto
		ctx := context.WithValue(r.Context(), CtxUserIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
