// Package devicepayload é a camada anticorrupção entre o firmware dos nós e o domínio
// (ADR 0007): converte o corpo bruto de cada lote (ADC, lux), que a bridge entrega dentro de um
// brutav1.TelemetriaBruta, em grandezas físicas com flag de qualidade, usando a calibração do
// sensor. O formato do corpo é o contrato contracts/mqtt/README.md.
//
// Cada versão de firmware tem um Translator registrado por versão maior (semver). Versões sem
// tradutor falham com UnsupportedFirmwareError em vez de serem convertidas por palpite.
package devicepayload
