// backend/handlers/auth_middleware.go
package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"backend/db"
)

// ctxKey es el tipo que usamos para guardar el userID en el contexto de la petición
type ctxKey string

const (
	CtxUserIDKey ctxKey = "userID"
)

// ControladorOnlyMiddleware verifica que el usuario que hace la petición tenga rol = 2 (“Controlador de los datos”).
// Se espera que el cliente incluya un header "X-User-ID: <numero>" en cada petició

func ControladorOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.Header.Get("X-User-ID")
		log.Println("Middleware: Header X-User-ID recibido:", idStr)
		if idStr == "" {
			http.Error(w, "No se indicó usuario autenticado", http.StatusUnauthorized)
			return
		}
		idUsuario, err := strconv.Atoi(idStr)
		if err != nil {
			log.Println("Middleware: ID inválido:", err)
			http.Error(w, "ID de usuario inválido", http.StatusBadRequest)
			return
		}

		log.Println("Middleware: Verificando rol controlador para usuario:", idUsuario)

		var exists int
		err = db.Pool.QueryRow(
			r.Context(),
			`
			SELECT 1
			FROM usuarios_roles
			WHERE id_usuario = $1
			AND id_rol = 2
			LIMIT 1
			`,
			idUsuario,
		).Scan(&exists)

		if err != nil {
			if err.Error() == "no rows in result set" {
				log.Println("Middleware: Acceso denegado, no tiene rol controlador")
				http.Error(w, "Acceso denegado: se requiere rol de Controlador", http.StatusForbidden)
				return
			}
			log.Println("Middleware: Error en consulta SQL:", err)
			http.Error(w, "Error interno al verificar rol", http.StatusInternalServerError)
			return
		}

		log.Println("Middleware: Acceso autorizado para usuario", idUsuario)
		ctx := context.WithValue(r.Context(), CtxUserIDKey, idUsuario)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserIDFromCtx es un helper para extraer el ID del usuario (int) desde el contexto
func GetUserIDFromCtx(ctx context.Context) (int, bool) {
	v := ctx.Value(CtxUserIDKey)
	if v == nil {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}
