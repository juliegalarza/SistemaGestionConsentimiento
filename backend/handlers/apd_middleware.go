// handlers/apd_middleware.go
package handlers

import (
	"backend/db"
	"context"
	"net/http"
	"strconv"
)

// AuthorityOnlyMiddleware verifica que el usuario tenga rol=5 (Autoridad de Protección de Datos)
func AuthorityOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Leer el ID de usuario del header
		idStr := r.Header.Get("X-User-ID")
		if idStr == "" {
			http.Error(w, "No se indicó usuario autenticado", http.StatusUnauthorized)
			return
		}
		// Convertir a entero
		idUsuario, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "ID de usuario inválido", http.StatusBadRequest)
			return
		}

		// Consultar en la tabla usuarios_roles si ese usuario tiene rol = 5
		var exists int
		err = db.Pool.QueryRow(
			r.Context(),
			`
            SELECT 1
              FROM usuarios_roles
             WHERE id_usuario = $1
               AND id_rol = 5
             LIMIT 1
            `,
			idUsuario,
		).Scan(&exists)

		if err != nil {
			// Si no hay filas, el usuario no tiene ese rol
			if err.Error() == "no rows in result set" {
				http.Error(w, "Acceso denegado: se requiere rol de Autoridad de Protección de Datos", http.StatusForbidden)
				return
			}
			// Otro error de BD
			http.Error(w, "Error interno al verificar rol", http.StatusInternalServerError)
			return
		}
		// Pasar el userID al contexto
		ctx := context.WithValue(r.Context(), CtxUserIDKey, idUsuario)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
