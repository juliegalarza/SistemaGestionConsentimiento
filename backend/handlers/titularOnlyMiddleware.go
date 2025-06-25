// TitularOnlyMiddleware verifica que el usuario tenga rol=1 (Titular)
package handlers

import (
	"backend/db"
	"context"
	"log"
	"net/http"
	"strconv"
)

func TitularOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.Header.Get("X-User-ID")
		log.Println("Middleware Titular: Header X-User-ID recibido:", idStr)
		if idStr == "" {
			http.Error(w, "No se indicó usuario autenticado", http.StatusUnauthorized)
			return
		}
		idUsuario, err := strconv.Atoi(idStr)
		if err != nil {
			log.Println("Middleware Titular: ID inválido:", err)
			http.Error(w, "ID de usuario inválido", http.StatusBadRequest)
			return
		}

		log.Println("Middleware Titular: Verificando rol titular para usuario:", idUsuario)

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
				log.Println("Middleware Titular: Acceso denegado, no tiene rol titular")
				http.Error(w, "Acceso denegado: se requiere rol de Titular", http.StatusForbidden)
				return
			}
			log.Println("Middleware Titular: Error en consulta SQL:", err)
			http.Error(w, "Error interno al verificar rol", http.StatusInternalServerError)
			return
		}

		log.Println("Middleware Titular: Acceso autorizado para usuario", idUsuario)
		ctx := context.WithValue(r.Context(), CtxUserIDKey, idUsuario)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
