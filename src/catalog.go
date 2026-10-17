package main

import (
	"sort"
	"strings"
)

// ControlDefinition describes an operational control exposed to integrations.
// The catalog is data-driven so operators can map telemetry to ownership
// without coupling the settlement engine to an alerting provider.
type ControlDefinition struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Severity  string   `json:"severity"`
	Component string   `json:"component"`
	Signal    string   `json:"signal"`
	Rationale string   `json:"rationale"`
	Response  string   `json:"response"`
	Owner     string   `json:"owner"`
	Tags      []string `json:"tags"`
}

type PlaybookStep struct {
	ID          string `json:"id"`
	Phase       string `json:"phase"`
	Trigger     string `json:"trigger"`
	Action      string `json:"action"`
	Expected    string `json:"expected"`
	Owner       string `json:"owner"`
	Automatable bool   `json:"automatable"`
}

var BuiltInScenarioNotes = map[string]string{
	"base_lifecycle":     "ciclo individual con entrega, cobro y cierre",
	"composite_closure":  "compromiso multibeneficiario liquidado por completo",
	"penalty_withdrawal": "vencimiento, penalizacion y control de retiro",
}

var ControlCatalog = []ControlDefinition{
	{
		ID: "ICD-ID-001", Title: "Identificadores unicos por dominio", Severity: "high",
		Component: "identity", Signal: "duplicate id rejected", Rationale: "Las entidades contables deben conservar una identidad estable durante todo su ciclo.",
		Response: "Rechazar la orden y conservar el evento de rechazo.", Owner: "platform",
		Tags: []string{"identity", "determinism"},
	},
	{
		ID: "ICD-AMT-001", Title: "Importes dentro del dominio permitido", Severity: "critical",
		Component: "amounts", Signal: "negative or overflowing amount", Rationale: "Cada transicion monetaria debe operar sobre enteros acotados.",
		Response: "Detener la transicion antes de modificar el estado.", Owner: "settlement",
		Tags: []string{"amount", "overflow"},
	},
	{
		ID: "ICD-VLT-001", Title: "Reservas no negativas", Severity: "critical",
		Component: "vaults", Signal: "reserve below zero", Rationale: "La reserva fisica es una magnitud monotona dentro de una transaccion.",
		Response: "Rechazar el lote y reconciliar el ultimo evento aplicado.", Owner: "treasury",
		Tags: []string{"reserve", "reconciliation"},
	},
	{
		ID: "ICD-VLT-002", Title: "Cobertura observada por activo", Severity: "high",
		Component: "vaults", Signal: "coverage ratio below configured corridor", Rationale: "La liquidez disponible debe compararse con las obligaciones activas por horizonte.",
		Response: "Escalar la revision de liquidez y recalcular los escenarios de estres.", Owner: "risk",
		Tags: []string{"coverage", "liquidity"},
	},
	{
		ID: "ICD-CMT-001", Title: "Suma de tramos reconciliada", Severity: "critical",
		Component: "commitments", Signal: "sum of beneficiary lanes differs from commitment amount", Rationale: "El notional agregado debe coincidir con el desglose contractual.",
		Response: "Rechazar la apertura y devolver un error determinista.", Owner: "settlement",
		Tags: []string{"commitment", "reconciliation"},
	},
	{
		ID: "ICD-CMT-002", Title: "Ventanas temporales monotonicas", Severity: "high",
		Component: "commitments", Signal: "due epoch precedes current epoch", Rationale: "La admision no debe crear ventanas vencidas de forma retroactiva.",
		Response: "Rechazar la orden y solicitar una nueva vigencia.", Owner: "operations",
		Tags: []string{"time", "admission"},
	},
	{
		ID: "ICD-DLV-001", Title: "Entrega acumulada acotada", Severity: "high",
		Component: "delivery", Signal: "delivered amount exceeds lane amount", Rationale: "Las pruebas de entrega no pueden aumentar el notional contratado.",
		Response: "Rechazar el exceso y mantener el acumulado anterior.", Owner: "settlement",
		Tags: []string{"delivery", "amount"},
	},
	{
		ID: "ICD-PEN-001", Title: "Penalizacion proporcional", Severity: "high",
		Component: "penalties", Signal: "penalty exceeds undelivered notional", Rationale: "La penalizacion se acota al importe pendiente del tramo.",
		Response: "Aplicar el minimo entre calculo de politica e importe pendiente.", Owner: "risk",
		Tags: []string{"penalty", "policy"},
	},
	{
		ID: "ICD-WDL-001", Title: "Retiro respaldado por saldo", Severity: "critical",
		Component: "withdrawals", Signal: "requested amount exceeds account reserve", Rationale: "Ningun retiro puede crear un saldo negativo.",
		Response: "Rechazar y registrar la solicitud sin efectos parciales.", Owner: "treasury",
		Tags: []string{"withdrawal", "reserve"},
	},
	{
		ID: "ICD-EVT-001", Title: "Secuencia de eventos continua", Severity: "medium",
		Component: "events", Signal: "event sequence contains a gap or duplicate", Rationale: "La reproduccion determinista depende de una secuencia total.",
		Response: "Detener la exportacion y reconstruir desde el ultimo punto consistente.", Owner: "platform",
		Tags: []string{"events", "observability"},
	},
	{
		ID: "ICD-RPT-001", Title: "Salida canonica", Severity: "medium",
		Component: "reporting", Signal: "same input produces different ordering", Rationale: "Los consumidores necesitan snapshots comparables byte a byte.",
		Response: "Ordenar colecciones por identificador antes de serializar.", Owner: "platform",
		Tags: []string{"reporting", "determinism"},
	},
	{
		ID: "ICD-OPS-001", Title: "Rechazos observables", Severity: "medium",
		Component: "operations", Signal: "rejected operation lacks code or field", Rationale: "La operacion diaria necesita causas estructuradas y accionables.",
		Response: "Emitir codigo, campo, indice y tipo de operacion.", Owner: "operations",
		Tags: []string{"operations", "observability"},
	},
}

var IncidentPlaybook = []PlaybookStep{
	{ID: "PB-001", Phase: "detect", Trigger: "automated control fails", Action: "capture the deterministic report and event cursor", Expected: "case has immutable input and output references", Owner: "operations", Automatable: true},
	{ID: "PB-002", Phase: "contain", Trigger: "reserve or amount control fails", Action: "pause new batches for the affected asset lane", Expected: "no new state transition is accepted for that lane", Owner: "treasury", Automatable: false},
	{ID: "PB-003", Phase: "measure", Trigger: "lane paused", Action: "reconcile accounts, vault reserve, fees and active claims", Expected: "reconciliation delta is quantified by asset", Owner: "risk", Automatable: true},
	{ID: "PB-004", Phase: "recover", Trigger: "reconciliation approved", Action: "prepare a reviewed compensating batch", Expected: "post-state returns to the approved corridor", Owner: "settlement", Automatable: false},
	{ID: "PB-005", Phase: "resume", Trigger: "controls and reconciliation pass", Action: "resume the lane with heightened monitoring", Expected: "two consecutive checkpoints remain within corridor", Owner: "operations", Automatable: false},
}

func ControlsBySeverity(severity string) []ControlDefinition {
	needle := strings.ToLower(strings.TrimSpace(severity))
	out := []ControlDefinition{}
	for _, control := range ControlCatalog {
		if strings.EqualFold(control.Severity, needle) {
			out = append(out, control)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ControlsByComponent(component string) []ControlDefinition {
	needle := strings.ToLower(strings.TrimSpace(component))
	out := []ControlDefinition{}
	for _, control := range ControlCatalog {
		if strings.EqualFold(control.Component, needle) {
			out = append(out, control)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ControlsByTag(tag string) []ControlDefinition {
	needle := strings.ToLower(strings.TrimSpace(tag))
	out := []ControlDefinition{}
	for _, control := range ControlCatalog {
		for _, candidate := range control.Tags {
			if strings.EqualFold(candidate, needle) {
				out = append(out, control)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func CriticalControlIDs() []string {
	controls := ControlsBySeverity("critical")
	ids := make([]string, 0, len(controls))
	for _, control := range controls {
		ids = append(ids, control.ID)
	}
	return ids
}

func ControlIDsForFinding(checkName string) []string {
	switch checkName {
	case "economic_claims_covered":
		return []string{"ICD-VLT-001", "ICD-VLT-002"}
	case "commitment_totals":
		return []string{"ICD-CMT-001"}
	case "no_negative_accounts":
		return []string{"ICD-AMT-001", "ICD-WDL-001"}
	case "no_negative_vaults":
		return []string{"ICD-AMT-001", "ICD-VLT-001"}
	default:
		return []string{}
	}
}

func PlaybookForPhase(phase string) []PlaybookStep {
	needle := strings.ToLower(strings.TrimSpace(phase))
	out := []PlaybookStep{}
	for _, step := range IncidentPlaybook {
		if strings.EqualFold(step.Phase, needle) {
			out = append(out, step)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ScenarioNote(name string) string {
	return BuiltInScenarioNotes[name]
}

func CatalogSummary() map[string]int {
	summary := map[string]int{}
	for _, control := range ControlCatalog {
		summary[control.Severity]++
		summary["component:"+control.Component]++
		for _, tag := range control.Tags {
			summary["tag:"+tag]++
		}
	}
	return summary
}
