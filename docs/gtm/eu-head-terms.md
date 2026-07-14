# EU head terms — the native procurement keyword per market

Living reference for tendersbay's 24-locale SEO: the head term SMEs actually type
when they look for public tenders in each market, plus the procurement-correct
"awarded" participle and the national portal that dominates that SERP.

**How it was gathered:** one scout per market ran native-language SERP checks
(tender-alert competitor queries, the national portal, award-notice terminology)
and recorded what official portals and commercial alert services optimize for.
Evidence is SERP-observational — no paid-tool search volumes were used, so treat
relative weight between terms as directional. The deep Italian dive lives in
[italian-keyword-map.md](italian-keyword-map.md).

**How to use it:** the primary term belongs in the technical/SEO spots of that
locale (meta title, meta description, hero highlight); secondaries go in
descriptions and body copy where they fit naturally; the awarded term is the only
acceptable "awarded" wording in technical spots (see the terminology split in
`.claude/memory/landing-page-design.md`). Locale files are ground truth for the
awarded participle — reconcile mismatches deliberately, never silently.

| Locale | Head term | Secondary | Awarded term | National portal | Trap / note |
| --- | --- | --- | --- | --- | --- |
| bg-bg | обществени поръчки | търгове, поръчки по ЗОП | възложена | ЦАИС ЕОП (aop.bg / app.eop.bg) | "търгове" also means auctions — keep secondary |
| cs-cz | veřejné zakázky | výběrová řízení, tendry | zadaná | NEN (nen.nipez.cz) | "výběrové řízení" also means job recruitment; "tendr" colloquial |
| da-dk | offentlige udbud | udbud, aktuelle udbud, EU-udbud | tildelt | udbud.dk | bare "udbud" also means supply/offering — qualify with "offentlige" |
| de-de | öffentliche Ausschreibungen | Ausschreibungen finden, öffentliche Aufträge, Vergabe | vergeben (notices); site copy uses "zugeschlagen" | service.bund.de | "Ausschreibung" alone can mean job postings; "Vergabe" is the jargon variant |
| el-gr | διαγωνισμοί δημοσίου | δημόσιοι διαγωνισμοί, δημόσιες συμβάσεις | κατακυρώθηκε | ΕΣΗΔΗΣ (promitheus.gov.gr) | "προκηρύξεις"/"διαγωνισμοί" alone mean job posts/contests — always pair with δημοσίου |
| en-ie | public tenders | government tenders Ireland, public sector contracts, tender opportunities | awarded | eTenders (etenders.gov.ie) | "bids/RFP" is US-leaning, not the Irish head term |
| es-es | licitaciones públicas | licitaciones, concursos públicos, contratación pública | adjudicado | PLACSP (contrataciondelestado.es) | "concursos" also means contests/insolvency; avoid "subastas" (auctions) |
| et-ee | riigihanked | hanked, hanketeated, avalikud hanked | sõlmitud (hankeleping sõlmitud) | Riigihangete register (riigihanked.riik.ee) | awards phrased as contract *signing* ("lepingu sõlmimise teade") |
| fi-fi | julkiset hankinnat | tarjouspyynnöt, kilpailutukset, hankintailmoitukset | tehty (hankintasopimus tehty) | Hilma (hankintailmoitukset.fi) | "tarjouskilpailu" alone can mean auctions/bidding generally |
| fr-fr | appels d'offres | marchés publics, appel d'offres public, avis de marché | attribué | BOAMP | "marché" alone is ambiguous (market); "adjugé" is auctions, not procurement |
| ga-ie | tairiscintí poiblí | soláthar poiblí, conarthaí rialtais | bronnta | eTenders (etenders.gov.ie) | Irish SMEs overwhelmingly search in English; "tairiscintí" alone = generic offers |
| hr-hr | javni natječaji | javna nabava, javna nadmetanja | dodijeljen | EOJN RH | "natječaj" alone covers job postings and grant calls — pair with "javni" |
| hu-hu | közbeszerzés | közbeszerzési pályázatok, közbeszerzés figyelés, tenderfigyelés | odaítélt | EKR (ekr.gov.hu) | bare "pályázat" primarily means grants/job applications |
| it-it | bandi di gara | gare d'appalto, appalti pubblici | aggiudicata | MEPA / acquistinretepa (Consip) | unqualified "bandi" means grants/job exams — always qualify |
| lt-lt | viešieji pirkimai | pirkimų skelbimai, viešųjų pirkimų konkursai | skirta (sutarties skyrimas) | CVP IS (viesiejipirkimai.lt / CVPP) | bare "konkursai" means job/creative competitions |
| lv-lv | publiskie iepirkumi | iepirkumi, valsts iepirkumi, izsludinātie iepirkumi | piešķirtas (līguma slēgšanas tiesības) | EIS (eis.gov.lv) + IUB notice search | "konkurss" is a grants/jobs ambiguity trap |
| mt-mt | sejħiet għall-offerti | offerti pubbliċi, government tenders Malta | mogħti | ePPS (etenders.gov.mt) | SMEs and competitors mostly target English "Malta tenders"; bare "offerti" = commercial offers |
| nl-nl | aanbestedingen | openbare aanbestedingen, overheidsopdrachten, tenders | gegund | TenderNed | no grant/auction ambiguity — clean head term |
| pl-pl | przetargi | zamówienia publiczne, wyszukiwarka przetargów, przetargi publiczne | udzielone | e-Zamówienia (ezamowienia.gov.pl) | "przetarg" can mean asset-sale auctions — procurement context disambiguates |
| pt-pt | concursos públicos | contratos públicos, contratação pública | adjudicado | Portal BASE (base.gov.pt) | "concursos públicos" also means public-sector job competitions — keep business context |
| ro-ro | licitații publice | licitații SEAP, achiziții publice, monitorizare licitații | atribuit | SEAP (e-licitatie.ro / SICAP) | "licitații" alone can mean auctions — pair with "publice"/"SEAP" |
| sk-sk | verejné obstarávanie | verejné zákazky, verejné súťaže, tendre | zadaná (zákazka zadaná) | ÚVO (uvo.gov.sk) / IS EPVO | avoid "dražby" (auctions) and "granty/dotácie" (grants) |
| sl-si | javna naročila | javni razpisi, razpisi za podjetja, aktualna javna naročila | oddano | Portal javnih naročil (enarocanje.si / eJN) | "javni razpisi" is high-volume but also means grant calls — secondary only |
| sv-se | offentliga upphandlingar | upphandlingar, bevaka upphandlingar, anbud | tilldelad | Mercell TendSign | "anbud" alone is any bid/offer (incl. real estate); "upphandling" is unambiguous |

Awarded-term reconciliation status vs the shipped meta copy: **bg-bg**
(възложени), **et-ee** (hankelepingu sõlmimiseni), **lt-lt** (sutarties
sudarymo), and **pl-pl** (rozstrzygnięte / udzielenie zamówienia) were aligned
to the native notice terms after adversarial locale review. **cs-cz**
(přidělené), **sk-sk** (udelené), and **sl-si** (dodeljena) deliberately keep
common-usage forms over the statute terms (zadaná/oddano) pending a native
check; **de-de** keeps "zugeschlagen" by standing design decision (see the
landing-page-design memory). The locale `common.json` files remain ground
truth.
