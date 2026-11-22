// Package devicepayload é a camada anticorrupção entre o firmware dos nós e o domínio
// (ADR 0007): converte o payload bruto de cada versão de firmware (ADC, lux) em grandezas
// físicas com flag de qualidade, usando a calibração do sensor.
//
// Cada versão de firmware tem um Translator registrado por versão maior (semver). Versões sem
// tradutor falham com UnsupportedFirmwareError em vez de serem convertidas por palpite.
package devicepayload
