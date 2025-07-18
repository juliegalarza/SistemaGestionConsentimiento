// TitularOnlyMiddleware verifica que el usuario tenga rol=1 (Titular)
package handlers

import (
	"backend/db"
	"context"
	"net/http"
	"strconv"
)

func TitularOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.Header.Get("X-User-ID")

		if idStr == "" {
			http.Error(w, "No se indicó usuario autenticado", http.StatusUnauthorized)
			return
		}
		idUsuario, err := strconv.Atoi(idStr)
		if err != nil {

			http.Error(w, "ID de usuario inválido", http.StatusBadRequest)
			return
		}

		var exists int
		err = db.Pool.QueryRow(
			r.Context(),
			`
            SELECT 1
            FROM usuarios_roles
            WHERE id_usuario = $1
              AND id_rol = 1
            LIMIT 1
            `,
			idUsuario,
		).Scan(&exists)

		if err != nil {
			if err.Error() == "no rows in result set" {

				http.Error(w, "Acceso denegado: se requiere rol de Titular", http.StatusForbidden)
				return
			}

			http.Error(w, "Error interno al verificar rol", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), CtxUserIDKey, idUsuario)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
