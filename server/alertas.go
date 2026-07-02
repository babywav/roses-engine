package main

import (
	"fmt"
	"log"
	"strings"
)

// Channel define os canais de entrega de alertas
type AlertChannel string

const (
	ChannelWhatsApp AlertChannel = "whatsapp"
	ChannelEmail    AlertChannel = "email"
	ChannelPush     AlertChannel = "push"
)

// AlertDispatcher orquestra a entrega multi-canal de notificações e alertas
type AlertDispatcher struct {
	WhatsApp WhatsAppProvider
	Email    EmailProvider
	Push     PushProvider
}

type WhatsAppProvider interface {
	SendWhatsApp(userID string, phone string, message string) error
}

type EmailProvider interface {
	SendEmail(userID string, email string, subject string, htmlBody string) error
}

type PushProvider interface {
	SendPush(userID string, title string, body string) error
}

// --- Provedores Simulados (Mock) para a Fase 3 (sem conexões de rede ativas) ---

type MockWhatsAppProvider struct{}

func (m *MockWhatsAppProvider) SendWhatsApp(userID string, phone string, message string) error {
	log.Printf("[ALERT WHATSAPP SIMULADO] Destinatário: %s | Usuário: %s", phone, userID)
	log.Printf("[ALERT WHATSAPP SIMULADO] Mensagem:\n%s\n=========================================", message)
	return nil
}

type MockEmailProvider struct{}

func (m *MockEmailProvider) SendEmail(userID string, email string, subject string, htmlBody string) error {
	log.Printf("[ALERT EMAIL SIMULADO] Destinatário: %s | Assunto: %s", email, subject)
	log.Printf("[ALERT EMAIL SIMULADO] Corpo HTML:\n%s\n=========================================", htmlBody)
	return nil
}

type MockPushProvider struct{}

func (m *MockPushProvider) SendPush(userID string, title string, body string) error {
	log.Printf("[ALERT PUSH SIMULADO] Título: %s | Corpo: %s", title, body)
	return nil
}

// GlobalDispatcher expõe o ponto único de disparo de alertas
var GlobalDispatcher = &AlertDispatcher{
	WhatsApp: &MockWhatsAppProvider{},
	Email:    &MockEmailProvider{},
	Push:     &MockPushProvider{},
}

// TriggerNewIntimacaoAlert dispara notificações nos três canais para uma nova intimação e sua minuta.
func TriggerNewIntimacaoAlert(userID string, processo string, tribunal string, tipoPeca string, vencimento string) {
	phone := "+55 (83) 99999-9999" // Mock carregado do perfil do advogado
	email := "advogado@roses.law"  // Mock carregado do perfil do advogado

	// 1. WhatsApp
	waMsg := fmt.Sprintf("Roses AI - Alerta de Prazo Oficial!\n\nProcesso: %s (%s)\nPeça Identificada: %s\nVencimento: %s (dias úteis)\n\n*A minuta sugerida de petição já foi criada automaticamente e está disponível no seu painel para revisão.*",
		processo, tribunal, tipoPeca, vencimento)
	_ = GlobalDispatcher.WhatsApp.SendWhatsApp(userID, phone, waMsg)

	// 2. Desktop Push Notification
	pushTitle := fmt.Sprintf("Nova intimação: %s", tipoPeca)
	pushBody := fmt.Sprintf("Prazo do processo %s vence em %s", processo, vencimento)
	_ = GlobalDispatcher.Push.SendPush(userID, pushTitle, pushBody)

	// 3. E-mail
	emailSubject := fmt.Sprintf("Roses AI - Novo Prazo Oficial - Processo %s", processo)
	emailBody := fmt.Sprintf(`
		<div style="font-family: sans-serif; padding: 20px; color: #333;">
			<h2 style="color: #6366f1;">Novo Prazo Identificado (DJEN)</h2>
			<p>Olá, Dr(a). Identificamos uma nova comunicação oficial publicada no DJEN e calculamos o prazo automaticamente:</p>
			<ul>
				<li><strong>Processo:</strong> %s</li>
				<li><strong>Tribunal:</strong> %s</li>
				<li><strong>Manifestação Sugerida:</strong> %s</li>
				<li><strong>Data de Vencimento:</strong> %s</li>
			</ul>
			<p>A minuta completa da peça processual foi rascunhada pela nossa inteligência artificial e está pronta para sua validação no painel do Roses.</p>
		</div>
	`, processo, tribunal, tipoPeca, vencimento)
	_ = GlobalDispatcher.Email.SendEmail(userID, email, emailSubject, emailBody)
}

// SendDailyDigestEmail compila um boletim de e-mail diário com todos os prazos ativos do usuário.
func SendDailyDigestEmail(userID string) {
	email := "advogado@roses.law"
	prazos := computePrazos(userID)

	var sb strings.Builder
	sb.WriteString("<div style='font-family: sans-serif; padding: 20px;'>")
	sb.WriteString("<h2 style='color: #6366f1;'>Boletim Diário Consolidado de Prazos - Roses</h2>")
	sb.WriteString("<p>Olá, Dr(a). Segue a lista consolidada de prazos processuais pendentes em sua carteira hoje:</p>")
	sb.WriteString("<table border='1' cellpadding='8' style='border-collapse: collapse; width: 100%; border-color: #ddd;'>")
	sb.WriteString("<tr style='background-color: #f3f4f6;'><th>Processo</th><th>Tribunal</th><th>Prazo</th><th>Vencimento</th><th>Status</th></tr>")

	count := 0
	for _, p := range prazos {
		if p.Status == "emdia" || p.Status == "urgente" || p.Status == "hoje" {
			count++
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
				p.Numero, p.Tribunal, p.Rotulo, p.Vencimento, p.Status))
		}
	}
	sb.WriteString("</table>")

	if count == 0 {
		sb.WriteString("<p>Nenhum prazo urgente ou hoje cadastrado para esta data.</p>")
	}
	sb.WriteString("</div>")

	_ = GlobalDispatcher.Email.SendEmail(userID, email, "Roses AI - Boletim Diário de Prazos", sb.String())
}
