// backend/handlers/custodio_middleware.go
package handlers

import (
	"context"
	"net/http"
	"strconv"

	"backend/db"
)

// ctxKey define un tipo privado para las claves del contexto
type ctxKey1 string

// CtxUserIDKey es la clave usada para almacenar el userID en el contexto
const CtxUserIDKey1 ctxKey1 = "userID"

// CustodioOnlyMiddleware asegura que sólo usuarios con rol “Custodio” (id_rol = 4)
// puedan acceder a las rutas protegidas con este middleware.
func CustodioOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Leer el ID de usuario desde el header (X-User-ID)
		idStr := r.Header.Get("X-User-ID")
		if idStr == "" {
			http.Error(w, "No autenticado", http.StatusUnauthorized)
			return
		}

		// 2. Convertir a entero
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "ID inválido", http.StatusBadRequest)
			return
		}

		// 3. Verificar en la BD que el usuario tenga el rol Custodio (id_rol=4)
		var exists int
		err = db.Pool.QueryRow(r.Context(), `
			SELECT 1 FROM usuarios_roles
			WHERE id_usuario = $1 AND id_rol = 4
			LIMIT 1
		`, id).Scan(&exists)
		if err != nil {
			http.Error(w, "Acceso denegado: se requiere rol Custodio", http.StatusForbidden)
			return
		}

		// 4. Añadir el userID al contexto para handlers posteriores
		ctx := context.WithValue(r.Context(), CtxUserIDKey1, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext extrae el userID del contexto, si existe.
func UserIDFromContext(ctx context.Context) (int, bool) {
	v := ctx.Value(CtxUserIDKey1)
	id, ok := v.(int)
	return id, ok
}
