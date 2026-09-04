package pdf

import (
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/codelist"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// labels is the PDF's copy in one language.
//
// The labels live HERE and not in the frontend's i18n bundles because this
// document is produced server-side and read by someone who may never open the
// app — a buyer's commission, a notary, the operator's own lawyer. Two
// languages ship: Italian, which is the market the product serves and the
// language an Italian commission expects, and English as the fallback for
// every other locale. A third language is a translation table away and should
// only be added when a real buyer asks, not preemptively.
type labels struct {
	title, subtitle                     string
	buyer, procedure, reference, notice string
	lots, version, composedAt           string
	answer, yes, no, selfCleaning       string
	declaredOn, confirmedOn             string
	partVI, declarationText             string
	signatureLine, notSigned, footer    string
	parts                               map[espd.Part]string
	criteria                            map[espd.CriterionKey]string
	fields                              map[string]string
}

func labelsFor(locale string) labels {
	if strings.HasPrefix(strings.ToLower(locale), "it") {
		return italian
	}
	return english
}

// part names a section, falling back to its bare identifier — an unlabelled
// part is a missing translation, and printing "IV.B" is honest about that where
// printing nothing would hide it.
func (l labels) part(p espd.Part) string {
	if s, ok := l.parts[p]; ok {
		return s
	}
	return string(p)
}

// criterion names a criterion, preferring this language's own wording and
// falling back to the OFFICIAL English name from the vendored code list rather
// than to the raw key. The EU already publishes that wording; duplicating it
// here would be a second source of truth for the same sentence.
func (l labels) criterion(k espd.CriterionKey) string {
	if s, ok := l.criteria[k]; ok {
		return s
	}
	if c, err := officialNames.Lookup(k); err == nil && c.Name != "" {
		return c.Name
	}
	return string(k)
}

// officialNames is the 2.1.1 table, read only for its criterion names. Either
// table would do — the names are the Directive's, not the version's.
var officialNames = codelist.EDM211()

func (l labels) field(f string) string {
	if s, ok := l.fields[f]; ok {
		return s
	}
	return f
}

var italian = labels{
	title:    "Documento di gara unico europeo (DGUE)",
	subtitle: "Parte I–VI, Regolamento di esecuzione (UE) 2016/7",
	buyer:    "Amministrazione aggiudicatrice", procedure: "Procedura",
	reference: "Riferimento (CIG)", notice: "Bando", lots: "Lotti",
	version: "Modello ESPD", composedAt: "Composto il",
	answer: "Risposta", yes: "Sì", no: "No", selfCleaning: "Misure di self-cleaning",
	declaredOn:  "Dichiarato il %s",
	confirmedOn: "Dichiarazioni della Parte III riconfermate per questa gara il %s.",
	partVI:      "Parte VI — Dichiarazioni finali",
	declarationText: "Il sottoscritto dichiara formalmente che le informazioni riportate " +
		"nelle parti II–V sono veritiere e corrette e che sono state fornite nella piena " +
		"consapevolezza delle conseguenze di una grave falsa dichiarazione. Il sottoscritto " +
		"dichiara di essere in grado, su richiesta e senza indugio, di presentare i certificati " +
		"e le altre prove documentali pertinenti.",
	signatureLine: "Luogo e data ______________________    Firma del legale rappresentante ______________________",
	notSigned: "Questo documento non è firmato. Va firmato digitalmente dal legale rappresentante " +
		"prima dell'invio alla stazione appaltante.",
	footer: "Preparato con tendersbay — da firmare digitalmente dal legale rappresentante",
	parts: map[espd.Part]string{
		espd.PartIIA:  "Parte II.A — Informazioni sull'operatore economico",
		espd.PartIIB:  "Parte II.B — Rappresentanti dell'operatore economico",
		espd.PartIIC:  "Parte II.C — Affidamento sulle capacità di altri soggetti",
		espd.PartIID:  "Parte II.D — Subappaltatori",
		espd.PartIIIA: "Parte III.A — Motivi legati a condanne penali",
		espd.PartIIIB: "Parte III.B — Pagamento di imposte e contributi previdenziali",
		espd.PartIIIC: "Parte III.C — Insolvenza, conflitti di interessi, illeciti professionali",
		espd.PartIIID: "Parte III.D — Motivi di esclusione previsti dalla normativa nazionale",
		espd.PartIVA:  "Parte IV.A — Idoneità",
		espd.PartIVB:  "Parte IV.B — Capacità economica e finanziaria",
		espd.PartIVC:  "Parte IV.C — Capacità tecniche e professionali",
		espd.PartIVD:  "Parte IV.D — Sistemi di garanzia della qualità e gestione ambientale",
	},
	criteria: map[espd.CriterionKey]string{
		espd.CritParticipationCriminalOrg: "Partecipazione a un'organizzazione criminale",
		espd.CritCorruption:               "Corruzione",
		espd.CritFraud:                    "Frode",
		espd.CritTerroristOffences:        "Reati terroristici o connessi ad attività terroristiche",
		espd.CritMoneyLaundering:          "Riciclaggio di proventi di attività criminose o finanziamento del terrorismo",
		espd.CritChildLabour:              "Lavoro minorile e altre forme di tratta di esseri umani",
		espd.CritPaymentTaxes:             "Pagamento di imposte e tasse",
		espd.CritPaymentSocialSecurity:    "Pagamento di contributi previdenziali",
		espd.CritEnvironmentalLaw:         "Violazione di obblighi in materia di diritto ambientale",
		espd.CritSocialLaw:                "Violazione di obblighi in materia di diritto sociale",
		espd.CritLabourLaw:                "Violazione di obblighi in materia di diritto del lavoro",
		espd.CritBankruptcy:               "Fallimento",
		espd.CritInsolvency:               "Insolvenza",
		espd.CritCreditorsArrangement:     "Concordato preventivo con i creditori",
		espd.CritAnalogousSituation:       "Situazione analoga al fallimento prevista dalla normativa nazionale",
		espd.CritAssetsByLiquidator:       "Beni amministrati da un liquidatore",
		espd.CritActivitiesSuspended:      "Attività commerciali sospese",
		espd.CritProfessionalMisconduct:   "Gravi illeciti professionali",
		espd.CritDistortingCompetition:    "Accordi con altri operatori economici volti a falsare la concorrenza",
		espd.CritConflictOfInterest:       "Conflitto di interessi dovuto alla partecipazione alla procedura",
		espd.CritInvolvementInPreparation: "Coinvolgimento diretto o indiretto nella preparazione della procedura",
		espd.CritEarlyTermination:         "Risoluzione anticipata, risarcimento danni o altre sanzioni comparabili",
		espd.CritMisrepresentation:        "False dichiarazioni, informazioni omesse, impossibilità di produrre documenti",
		espd.CritIdentity:                 "Identificazione",
		espd.CritRepresentatives:          "Rappresentante",
		espd.CritReliance:                 "Affidamento sulle capacità di altri soggetti",
		espd.CritSubcontracting:           "Subappalto",
		espd.CritEnrolmentProfessionalReg: "Iscrizione in un albo professionale",
		espd.CritEnrolmentTradeReg:        "Iscrizione nel registro delle imprese",
		espd.CritOtherRegistration:        "Altre iscrizioni e autorizzazioni",
		espd.CritGeneralTurnover:          "Fatturato annuo generale",
		espd.CritSpecificTurnover:         "Fatturato annuo specifico di settore",
		espd.CritReferences:               "Referenze — lavori, forniture o servizi eseguiti",
		espd.CritAverageAnnualManpower:    "Organico medio annuo",
		espd.CritSOAAttestation:           "Attestazione SOA (iscrizione in elenco ufficiale)",
		espd.CritQualityAssurance:         "Certificati di garanzia della qualità",
		espd.CritEnvironmentalManagement:  "Certificati di gestione ambientale",
	},
	fields: map[string]string{
		"legal_name": "Denominazione", "vat_number": "Partita IVA", "fiscal_code": "Codice fiscale",
		"legal_form": "Forma giuridica", "country": "Paese", "nuts": "Codice NUTS", "is_sme": "PMI",
		"role": "Ruolo", "given_name": "Nome", "family_name": "Cognome",
		"birth_date": "Data di nascita", "birth_place": "Luogo di nascita",
		"address": "Indirizzo", "email": "Email", "power_of_attorney": "Procura",
		"entity_name": "Denominazione del soggetto", "vat": "Partita IVA", "criterion": "Requisito",
		"relies_on_other_entities": "Ricorso a capacità di altri soggetti",
		"intends_to_subcontract":   "Intenzione di subappaltare",
		"name":                     "Denominazione", "share_pct": "Quota (%)",
		"register": "Registro", "office": "Ufficio", "number": "Numero",
		"authority": "Autorità", "section": "Sezione", "valid_until": "Valido fino al",
		"year": "Esercizio", "amount": "Importo", "headcount": "Organico",
		"description": "Descrizione", "recipient": "Committente",
		"started_on": "Inizio", "ended_on": "Fine",
		"category": "Categoria", "classifica": "Classifica", "issued_by": "Rilasciato da",
		"standard": "Norma", "scope": "Ambito", "lot_ref": "Lotto",
	},
}

var english = labels{
	title:    "European Single Procurement Document (ESPD)",
	subtitle: "Parts I–VI, Commission Implementing Regulation (EU) 2016/7",
	buyer:    "Contracting authority", procedure: "Procedure",
	reference: "Reference", notice: "Notice", lots: "Lots",
	version: "ESPD model", composedAt: "Composed on",
	answer: "Answer", yes: "Yes", no: "No", selfCleaning: "Self-cleaning measures",
	declaredOn:  "Declared on %s",
	confirmedOn: "Part III declarations re-confirmed for this procedure on %s.",
	partVI:      "Part VI — Concluding statements",
	declarationText: "The undersigned formally declares that the information stated in Parts II–V " +
		"is accurate and correct and that it has been set out in full awareness of the consequences " +
		"of serious misrepresentation. The undersigned formally declares to be able, upon request " +
		"and without delay, to provide the certificates and other forms of documentary evidence referred to.",
	signatureLine: "Place and date ______________________    Signature of the legal representative ______________________",
	notSigned: "This document is not signed. It must be signed electronically by the legal " +
		"representative before it is submitted to the contracting authority.",
	footer: "Prepared with tendersbay — to be signed electronically by the legal representative",
	parts: map[espd.Part]string{
		espd.PartIIA:  "Part II.A — Information about the economic operator",
		espd.PartIIB:  "Part II.B — Representatives of the economic operator",
		espd.PartIIC:  "Part II.C — Reliance on the capacities of other entities",
		espd.PartIID:  "Part II.D — Subcontractors",
		espd.PartIIIA: "Part III.A — Grounds relating to criminal convictions",
		espd.PartIIIB: "Part III.B — Payment of taxes and social security contributions",
		espd.PartIIIC: "Part III.C — Insolvency, conflicts of interest, professional misconduct",
		espd.PartIIID: "Part III.D — Purely national exclusion grounds",
		espd.PartIVA:  "Part IV.A — Suitability",
		espd.PartIVB:  "Part IV.B — Economic and financial standing",
		espd.PartIVC:  "Part IV.C — Technical and professional ability",
		espd.PartIVD:  "Part IV.D — Quality assurance and environmental management",
	},
	// English criterion names come from the vendored code list — see
	// labels.criterion. Only the field labels the ESPD does not name are here.
	criteria: map[espd.CriterionKey]string{},
	fields: map[string]string{
		"legal_name": "Name", "vat_number": "VAT number", "fiscal_code": "Fiscal code",
		"legal_form": "Legal form", "country": "Country", "nuts": "NUTS code", "is_sme": "SME",
		"given_name": "First name", "family_name": "Family name",
		"birth_date": "Date of birth", "birth_place": "Place of birth",
		"power_of_attorney": "Power of attorney",
		"entity_name":       "Entity name", "vat": "VAT number",
		"relies_on_other_entities": "Relies on other entities",
		"intends_to_subcontract":   "Intends to subcontract",
		"share_pct":                "Share (%)", "valid_until": "Valid until",
		"started_on": "Start", "ended_on": "End", "lot_ref": "Lot",
	},
}
