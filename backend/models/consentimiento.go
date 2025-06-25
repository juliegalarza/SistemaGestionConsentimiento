package models

import "time"

type Consentimiento struct {
	IDConsentimiento  int        `json:"id_consentimiento"`
	IDUsuario         int        `json:"id_usuario"`
	IDPolitica        int        `json:"id_politica"`
	FechaOtorgado     time.Time  `json:"fecha_otorgado"`
	FechaExpiracion   *time.Time `json:"fecha_expiracion"`
	Estado            string     `json:"estado"`
	RevocadoPendiente bool       `json:"revocado_pendiente"`
	ReferenciaID      int        `json:"referencia_id"`
}
