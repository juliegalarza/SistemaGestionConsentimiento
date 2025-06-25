// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"backend/db"
	"backend/handlers"
	"backend/utils"
)

func habilitarCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// 0️⃣ Conexión a las bases de datos
	db.ConectarDB()
	defer db.Pool.Close()
	db.ConectarDatosPersonales()

	// 1️⃣ Inicializar ABE
	utils.InicializarABE()

	// 2️⃣ Configurar router y rutas
	r := mux.NewRouter()

	// — PÚBLICAS —
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Backend funcionando correctamente")
	}).Methods("GET")
	r.HandleFunc("/registro", handlers.RegistrarUsuario).Methods("POST")
	r.HandleFunc("/login", handlers.LoginUsuario).Methods("POST")
	r.HandleFunc("/usuarios", handlers.ObtenerUsuarios).Methods("GET")

	// — CONTROLADOR (rol = 4) —
	ctd := r.PathPrefix("/custodio").Subrouter()
	ctd.Use(handlers.CustodioOnlyMiddleware)

	// • Notificaciones (controlador)
	ctd.HandleFunc("/notificaciones", handlers.GetNotificaciones).Methods("GET")
	ctd.HandleFunc("/notificaciones/count", handlers.GetUnreadCount).Methods("GET")
	ctd.HandleFunc("/notificaciones/{id}/leer", handlers.MarkAsRead).Methods("PUT")
	ctd.HandleFunc("/accesos", handlers.ObtenerAccesosCustodio).Methods("GET")
	ctd.HandleFunc("/consentimientos", handlers.ObtenerConsentimientosCustodio).Methods("GET")
	// — CONTROLADOR (rol = 2) —
	ctrl := r.PathPrefix("/controlador").Subrouter()
	ctrl.Use(handlers.ControladorOnlyMiddleware)

	// • Gestión de roles
	ctrl.HandleFunc("/usuarios-roles", handlers.CrearUsuarioRol).Methods("POST")
	ctrl.HandleFunc("/usuarios-roles", handlers.ObtenerRolesUsuario).Methods("GET")
	ctrl.HandleFunc("/usuarios-roles", handlers.EliminarUsuarioRol).Methods("DELETE")

	// • Usuarios procesadores
	ctrl.HandleFunc("/usuarios-procesadores", handlers.ObtenerUsuariosProcesadores).Methods("GET")

	// • Atributos de terceros
	ctrl.HandleFunc("/atributos-terceros", handlers.AsignarAtributosTercero).Methods("POST")
	ctrl.HandleFunc("/atributos-terceros", handlers.ObtenerAtributosDeTercero).Methods("GET")
	ctrl.HandleFunc("/atributos-terceros", handlers.ActualizarAtributosTercero).Methods("PUT")
	ctrl.HandleFunc("/atributos-terceros", handlers.EliminarAtributosTercero).Methods("DELETE")
	ctrl.HandleFunc("/usuarios/{id}", handlers.ObtenerUsuarioPorID).Methods("GET")

	// • Políticas de privacidad (controlador)
	// Gestión de políticas de privacidad
	ctrl.HandleFunc("/politicas-privacidad", handlers.ObtenerPoliticasParaControlador).Methods("GET")
	ctrl.HandleFunc("/politicas-privacidad", handlers.CrearPoliticaControlador).Methods("POST")
	// Cambia estas dos líneas:
	ctrl.HandleFunc("/politicas-privacidad/{id_politica}", handlers.ActualizarPoliticaControlador).Methods("PUT")
	ctrl.HandleFunc("/politicas-privacidad/{id_politica}", handlers.EliminarPoliticaControlador).Methods("DELETE")
	ctrl.HandleFunc("/politicas-privacidad/{id_politica}", handlers.ObtenerPoliticaPorIDC).Methods("GET")

	// • Asignación de atributos a política
	ctrl.HandleFunc("/politica-atributos", handlers.ObtenerAtributosDePolitica).Methods("GET")
	ctrl.HandleFunc("/atributos-datos", handlers.ObtenerAtributosDatos).Methods("GET")

	// • Consentimientos (monitoreo)
	ctrl.HandleFunc("/consentimientos", handlers.ObtenerConsentimientos).Methods("GET")
	ctrl.HandleFunc("/monitoreo-consentimientos", handlers.MonitorConsentimientos).Methods("GET")

	// • Dashboard controlador
	ctrl.HandleFunc("/dashboard", handlers.Dashboard).Methods("GET")

	// • Notificaciones (controlador)
	ctrl.HandleFunc("/notificaciones", handlers.GetNotificaciones).Methods("GET")
	ctrl.HandleFunc("/notificaciones/count", handlers.GetUnreadCount).Methods("GET")
	ctrl.HandleFunc("/notificaciones/{id}/leer", handlers.MarkAsRead).Methods("PUT")

	//Asignar Rol
	ctrl.HandleFunc("/usuarios-sin-rol", handlers.ObtenerUsuariosSinRol).Methods("GET")
	ctrl.HandleFunc("/usuarios-roles", handlers.CrearUsuarioRol).Methods("POST")
	ctrl.HandleFunc("/usuarios-roles", handlers.ObtenerRolesUsuario).Methods("GET")
	ctrl.HandleFunc("/usuarios-roles", handlers.EliminarUsuarioRol).Methods("DELETE")
	ctrl.HandleFunc("/roles", handlers.ObtenerRoles).Methods("GET")
	ctrl.HandleFunc("/usuarios-asignables", handlers.ObtenerUsuariosAsignables).Methods("GET")
	ctrl.HandleFunc("/solicitudes-atributo", handlers.ObtenerSolicitudesAtributo).Methods("GET")
	ctrl.HandleFunc("/solicitudes-atributo/{id}", handlers.ObtenerSolicitudAtributoPorID).Methods("GET")
	ctrl.HandleFunc("/solicitudes-atributo/{id}", handlers.ActualizarEstadoSolicitudAtributo).Methods("PUT")
	ctrl.HandleFunc("/atributos-terceros", handlers.ObtenerAtributosDeTercero).Methods("GET")
	// — TITULAR (rol = 1) —
	tit := r.PathPrefix("/titular").Subrouter()
	tit.Use(handlers.TitularOnlyMiddleware)

	// Datos personales
	tit.HandleFunc("/datos-personales", handlers.GuardarDatosPersonales).Methods("POST")
	tit.HandleFunc("/datos-personales", handlers.ObtenerDatosPersonales).Methods("GET")
	tit.HandleFunc("/datos-personales", handlers.ActualizarDatosPersonales).Methods("PUT")
	tit.HandleFunc("/datos-personales", handlers.EliminarDatosPersonales).Methods("DELETE")

	// Políticas
	tit.HandleFunc("/politicas", handlers.ObtenerPoliticas).Methods("GET")

	// Consentimientos
	tit.HandleFunc("/consentimientos", handlers.GuardarConsentimiento).Methods("POST")
	tit.HandleFunc("/consentimientos", handlers.ObtenerConsentimientosPorUsuario).Methods("GET")
	tit.HandleFunc("/consentimientos", handlers.ActualizarConsentimiento).Methods("PUT")
	tit.HandleFunc("/consentimientos", handlers.EliminarConsentimiento).Methods("DELETE")
	tit.HandleFunc("/consentimientos/revocar", handlers.RevocarConsentimiento).Methods("POST")

	// Dashboard titular
	tit.HandleFunc("/dashboard", handlers.Dashboard).Methods("GET")

	// Notificaciones para el titular
	tit.HandleFunc("/notificaciones", handlers.GetNotificaciones).Methods("GET")
	tit.HandleFunc("/notificaciones/count", handlers.GetUnreadCount).Methods("GET")
	tit.HandleFunc("/notificaciones/{id}/leer", handlers.MarkAsRead).Methods("PUT")

	// --- Procesador (rol = 3) ---
	proc := r.PathPrefix("/procesador").Subrouter()
	proc.Use(handlers.ProcesadorOnlyMiddleware)

	// Dashboard del procesador

	// Endpoint de acceso a datos personales
	proc.HandleFunc("/acceso-datos", handlers.ObtenerAccesoDatos).Methods("GET")
	// Listado de políticas (o lo que uses en PoliticasProcesadorComponent)
	//proc.HandleFunc("/politicas-procesador", handlers.ObtenerPoliticasParaProcesador).Methods("GET")
	proc.HandleFunc("/atributos-terceros", handlers.ObtenerAtributosDeTercero).Methods("GET")
	proc.HandleFunc("/titulares-por-atributo", handlers.ObtenerTitularesPorAtributo).Methods("GET")
	proc.HandleFunc("/solicitudes-attributo", handlers.CrearSolicitudAtributoP).Methods("POST")
	proc.HandleFunc("/solicitudes-modificacion", handlers.CrearSolicitudModificacion).Methods("POST")
	proc.HandleFunc("/politicas", handlers.ObtenerPoliticasParaProcesador).Methods("GET")

	proc.HandleFunc("/todas-politicas", handlers.ObtenerTodasLasPoliticas).Methods("GET")
	proc.HandleFunc("/politica-atributos", handlers.ObtenerAtributosDesPolitica).Methods("GET")
	// • Notificaciones (controlador)
	proc.HandleFunc("/notificaciones", handlers.GetNotificaciones).Methods("GET")
	proc.HandleFunc("/notificaciones/count", handlers.GetUnreadCount).Methods("GET")
	proc.HandleFunc("/notificaciones/{id}/leer", handlers.MarkAsRead).Methods("PUT")

	// 3️⃣ Tareas background

	// 3.1) Generar notificaciones periódicas
	go func() {
		if err := handlers.GenerarNotificacionesConsentimientos(); err != nil {
			log.Println("Error generando notificaciones:", err)
		} else {
			log.Println("Notificaciones generadas correctamente")
		}
	}()

	// 3.2) Expirar consentimientos pasados de fecha
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			ctx := context.Background()
			if _, err := db.Pool.Exec(ctx, `
				UPDATE consentimientos
				   SET estado = 'expirado'
				 WHERE estado = 'activo'
				   AND fecha_expiracion < NOW()
			`); err != nil {
				log.Printf("Error actualizando expirados: %v", err)
			}
		}
	}()

	// 3.3) Efectivar revocaciones pendientes (>24h)
	go func() {
		ticker := time.NewTicker(1 * time.Minute) // para producción: 24 * time.Hour
		defer ticker.Stop()
		for range ticker.C {
			ctx := context.Background()
			const sql = `
WITH to_update AS (
  SELECT id_consentimiento
    FROM consentimientos
   WHERE revocado_pendiente = TRUE
     AND fecha_revocacion   < NOW() - INTERVAL '1 minute'
)
UPDATE consentimientos c
   SET estado            = 'revocado',
       revocado_pendiente = FALSE,
	   fecha_expiracion   = NOW()
  FROM to_update u
 WHERE c.id_consentimiento = u.id_consentimiento
RETURNING c.id_consentimiento;
`
			rows, err := db.Pool.Query(ctx, sql)
			if err != nil {
				log.Printf("Error efectivando revocaciones: %v", err)
				continue
			}
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err != nil {
					log.Printf("Error leyendo revocación efectuada: %v", err)
					continue
				}
				log.Printf("Consentimiento %d marcado como revocado final", id)
				// Aquí puedes disparar notificaciones finales si lo deseas
			}
			rows.Close()
		}
	}()

	// 4️⃣ Arrancar servidor
	log.Println("Servidor corriendo en http://localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", habilitarCORS(r)))
}
