---
title: Zwischenspeicherung
weight: 6
---

CC-Relay enthaelt eine flexible Caching-Schicht, die Latenz und Backend-Last durch Zwischenspeichern von Antworten von LLM-Providern erheblich reduzieren kann.

## Uebersicht

Das Cache-Subsystem unterstuetzt drei Betriebsmodi:

| Modus | Backend | Beschreibung |
|-------|---------|--------------|
| `single` | Ristretto | Hochleistungs-In-Memory-Cache fuer einzelne Knoten (Standard) |
| `ha` | Olric | Verteilter Cache fuer Hochverfuegbarkeits-Deployments |
| `disabled` | Noop | Durchleitungsmodus ohne Caching |

**Wann welchen Modus verwenden:**

- **Single-Modus**: Entwicklung, Tests oder Einzel-Instanz-Produktions-Deployments. Bietet die niedrigste Latenz ohne Netzwerk-Overhead.
- **HA-Modus**: Multi-Instanz-Produktions-Deployments, bei denen Cache-Konsistenz ueber Knoten hinweg erforderlich ist.
- **Disabled-Modus**: Debugging, Compliance-Anforderungen oder wenn Caching anderweitig gehandhabt wird.

## Architektur

```mermaid
graph TB
    subgraph "cc-relay"
        A[Proxy Handler] --> B{Cache Layer}
        B --> C[Cache Interface]
    end

    subgraph "Backends"
        C --> D[Ristretto<br/>Single Node]
        C --> E[Olric<br/>Distributed]
        C --> F[Noop<br/>Disabled]
    end

    style A fill:#6366f1,stroke:#4f46e5,color:#fff
    style B fill:#ec4899,stroke:#db2777,color:#fff
    style C fill:#f59e0b,stroke:#d97706,color:#000
    style D fill:#10b981,stroke:#059669,color:#fff
    style E fill:#8b5cf6,stroke:#7c3aed,color:#fff
    style F fill:#6b7280,stroke:#4b5563,color:#fff
```

Die Cache-Schicht implementiert ein einheitliches `Cache`-Interface, das alle Backends abstrahiert:

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte) error
    SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    Close() error
}
```

## Cache-Ablauf

```mermaid
sequenceDiagram
    participant Client
    participant Proxy
    participant Cache
    participant Backend

    Client->>Proxy: POST /v1/messages
    Proxy->>Cache: Get(key)
    alt Cache Hit
        Cache-->>Proxy: Cached Response
        Proxy-->>Client: Response (fast)
        Note over Client,Proxy: Latency: ~1ms
    else Cache Miss
        Cache-->>Proxy: ErrNotFound
        Proxy->>Backend: Forward Request
        Backend-->>Proxy: LLM Response
        Proxy->>Cache: SetWithTTL(key, value, ttl)
        Proxy-->>Client: Response
        Note over Client,Backend: Latency: 500ms-30s
    end
```

## Konfiguration

### Single-Modus (Ristretto)

Ristretto ist ein hochleistungsfaehiger, nebenlaeufiger Cache basierend auf Forschung der Caffeine-Bibliothek. Er verwendet eine TinyLFU-Admission-Policy fuer optimale Trefferquoten.

```yaml
cache:
  mode: single

  ristretto:
    # Anzahl der 4-Bit-Zugriffszaehler
    # Empfohlen: 10x der erwarteten maximalen Eintraege fuer optimale Admission-Policy
    # Beispiel: Fuer 100.000 Eintraege, verwenden Sie 1.000.000 Zaehler
    num_counters: 1000000

    # Maximaler Speicher fuer zwischengespeicherte Werte (in Bytes)
    # 104857600 = 100 MB
    max_cost: 104857600

    # Anzahl der Schluessel pro Get-Buffer (Standard: 64)
    # Steuert die Groesse des Admission-Buffers
    buffer_items: 64
```

**Speicherberechnung:**

Der `max_cost`-Parameter steuert, wie viel Speicher der Cache fuer Werte verwenden kann. Um die angemessene Groesse zu schaetzen:

1. Schaetzen Sie die durchschnittliche Antwortgroesse (typischerweise 1-10 KB fuer LLM-Antworten)
2. Multiplizieren Sie mit der Anzahl der eindeutigen Anfragen, die Sie zwischenspeichern moechten
3. Fuegen Sie 20% Overhead fuer Metadaten hinzu

Beispiel: 10.000 zwischengespeicherte Antworten x 5 KB Durchschnitt = 50 MB, also setzen Sie `max_cost: 52428800`

### HA-Modus (Olric)

Olric bietet verteiltes Caching mit automatischer Cluster-Erkennung und Datenreplikation.

**Client-Modus** (Verbindung zu externem Cluster):

```yaml
cache:
  mode: ha

  olric:
    # Olric-Cluster-Mitglieder-Adressen
    addresses:
      - "olric-1:3320"
      - "olric-2:3320"
      - "olric-3:3320"

    # Name der verteilten Map (Standard: "cc-relay")
    dmap_name: "cc-relay"
```

**Embedded-Modus** (Einzel-Knoten-HA oder Entwicklung):

```yaml
cache:
  mode: ha

  olric:
    # Eingebetteten Olric-Knoten ausfuehren
    embedded: true

    # Adresse fuer den eingebetteten Knoten
    bind_addr: "0.0.0.0:3320"

    # Peer-Adressen fuer Cluster-Erkennung (optional)
    peers:
      - "cc-relay-2:3320"
      - "cc-relay-3:3320"

    dmap_name: "cc-relay"
```

### Disabled-Modus

```yaml
cache:
  mode: disabled
```

Alle Cache-Operationen kehren sofort zurueck, ohne Daten zu speichern. `Get`-Operationen geben immer `ErrNotFound` zurueck.

## Vergleich der Cache-Modi

| Funktion | Single (Ristretto) | HA (Olric) | Disabled (Noop) |
|----------|-------------------|------------|-----------------|
| **Backend** | Lokaler Speicher | Verteilt | Keines |
| **Anwendungsfall** | Entwicklung, Einzel-Instanz | Produktion HA | Debugging |
| **Persistenz** | Nein | Optional | N/A |
| **Multi-Knoten** | Nein | Ja | N/A |
| **Latenz** | ~1 Mikrosekunde | ~1-10 ms (Netzwerk) | ~0 |
| **Speicher** | Nur lokal | Verteilt | Keiner |
| **Konsistenz** | N/A | Eventual | N/A |
| **Komplexitaet** | Niedrig | Mittel | Keine |

## Optionale Schnittstellen

Einige Cache-Backends unterstuetzen zusaetzliche Faehigkeiten ueber optionale Schnittstellen:

### Statistiken

```go
if sp, ok := cache.(cache.StatsProvider); ok {
    stats := sp.Stats()
    fmt.Printf("Hits: %d, Misses: %d\n", stats.Hits, stats.Misses)
}
```

Statistiken umfassen:
- `Hits`: Anzahl der Cache-Treffer
- `Misses`: Anzahl der Cache-Fehlschlaege
- `KeyCount`: Aktuelle Anzahl der Schluessel
- `BytesUsed`: Ungefaehrer verwendeter Speicher
- `Evictions`: Wegen Kapazitaet entfernte Schluessel

### Gesundheitspruefung (Ping)

```go
if p, ok := cache.(cache.Pinger); ok {
    if err := p.Ping(ctx); err != nil {
        // Cache ist nicht gesund
    }
}
```

Die `Pinger`-Schnittstelle ist hauptsaechlich fuer verteilte Caches (Olric) nuetzlich, um die Cluster-Konnektivitaet zu ueberpruefen.

### Batch-Operationen

```go
// Batch-Get
if mg, ok := cache.(cache.MultiGetter); ok {
    results, err := mg.GetMulti(ctx, []string{"key1", "key2", "key3"})
}

// Batch-Set
if ms, ok := cache.(cache.MultiSetter); ok {
    err := ms.SetMultiWithTTL(ctx, items, 5*time.Minute)
}
```

## Performance-Tipps

### Ristretto optimieren

1. **`num_counters` angemessen setzen**: Verwenden Sie 10x Ihrer erwarteten maximalen Eintraege. Zu niedrig reduziert die Trefferquote; zu hoch verschwendet Speicher.

2. **`max_cost` basierend auf Antwortgroessen dimensionieren**: LLM-Antworten variieren stark. Ueberwachen Sie die tatsaechliche Nutzung und passen Sie an.

3. **TTL weise verwenden**: Kurze TTLs (1-5 Min) fuer dynamischen Inhalt, laengere TTLs (1 Stunde+) fuer deterministische Antworten.

4. **Metriken ueberwachen**: Verfolgen Sie die Trefferquote, um die Cache-Effektivitaet zu validieren:
   ```
   hit_rate = hits / (hits + misses)
   ```
   Streben Sie >80% Trefferquote fuer effektives Caching an.

### Olric optimieren

1. **Nahe an cc-relay-Instanzen bereitstellen**: Netzwerklatenz dominiert die Performance verteilter Caches.

2. **Embedded-Modus fuer Einzel-Knoten-Deployments verwenden**: Vermeidet externe Abhaengigkeiten bei Beibehaltung der HA-faehigen Konfiguration.

3. **Cluster angemessen dimensionieren**: Jeder Knoten sollte genuegend Speicher fuer den vollstaendigen Datensatz haben (Olric repliziert Daten).

4. **Cluster-Gesundheit ueberwachen**: Verwenden Sie die `Pinger`-Schnittstelle in Gesundheitspruefungen.

### Allgemeine Tipps

1. **Cache-Schluessel-Design**: Verwenden Sie deterministische Schluessel basierend auf Anfrageinhalten. Schliessen Sie Modellname, Prompt-Hash und relevante Parameter ein.

2. **Streaming-Antworten nicht zwischenspeichern**: Streaming-SSE-Antworten werden standardmaessig aufgrund ihrer inkrementellen Natur nicht zwischengespeichert.

3. **Cache-Aufwaermung erwaegen**: Fuer vorhersehbare Workloads den Cache mit haeufigen Abfragen vorausfuellen.

## Fehlerbehebung

### Cache-Fehlschlaege bei erwarteten Treffern

1. **Schluesselgenerierung pruefen**: Stellen Sie sicher, dass Cache-Schluessel deterministisch sind und keine Zeitstempel oder Anfrage-IDs enthalten.

2. **TTL-Einstellungen ueberpruefen**: Eintraege koennten abgelaufen sein. Pruefen Sie, ob die TTL fuer Ihren Anwendungsfall zu kurz ist.

3. **Evictions ueberwachen**: Hohe Eviction-Zaehler zeigen an, dass `max_cost` zu niedrig ist:
   ```go
   stats := cache.Stats()
   if stats.Evictions > 0 {
       // Erwaegen Sie, max_cost zu erhoehen
   }
   ```

### Ristretto speichert keine Eintraege

Ristretto verwendet eine Admission-Policy, die Eintraege ablehnen kann, um hohe Trefferquoten zu erhalten. Dies ist normales Verhalten:

1. **Neue Eintraege koennen abgelehnt werden**: TinyLFU erfordert, dass Eintraege ihren Wert durch wiederholten Zugriff "beweisen".

2. **Auf Buffer-Flush warten**: Ristretto puffert Schreibvorgaenge. Rufen Sie `cache.Wait()` in Tests auf, um sicherzustellen, dass Schreibvorgaenge verarbeitet werden.

3. **Kostenberechnung pruefen**: Eintraege mit Kosten > `max_cost` werden nie gespeichert.

### Olric-Cluster-Konnektivitaetsprobleme

1. **Netzwerkkonnektivitaet ueberpruefen**: Stellen Sie sicher, dass alle Knoten sich gegenseitig auf Port 3320 (oder konfiguriertem Port) erreichen koennen.

2. **Firewall-Regeln pruefen**: Olric erfordert bidirektionale Kommunikation zwischen Knoten.

3. **Adressen validieren**: Im Client-Modus stellen Sie sicher, dass mindestens eine Adresse in der Liste erreichbar ist.

4. **Logs ueberwachen**: Aktivieren Sie Debug-Protokollierung, um Cluster-Mitgliedschafts-Ereignisse zu sehen:
   ```yaml
   logging:
     level: debug
   ```

### Speicherdruck

1. **`max_cost` reduzieren**: Verringern Sie die Cache-Groesse, um den Speicherverbrauch zu reduzieren.

2. **Kuerzere TTLs verwenden**: Eintraege schneller ablaufen lassen, um Speicher freizugeben.

3. **Zu Olric wechseln**: Speicherdruck auf mehrere Knoten verteilen.

4. **Mit Metriken ueberwachen**: Verfolgen Sie `BytesUsed`, um den tatsaechlichen Speicherverbrauch zu verstehen.

## Fehlerbehandlung

Das Cache-Paket definiert Standardfehler fuer haeufige Bedingungen:

```go
import "github.com/anthropics/cc-relay/internal/cache"

data, err := c.Get(ctx, key)
switch {
case errors.Is(err, cache.ErrNotFound):
    // Cache-Fehlschlag - vom Backend abrufen
case errors.Is(err, cache.ErrClosed):
    // Cache wurde geschlossen - neu erstellen oder fehlschlagen
case err != nil:
    // Anderer Fehler (Netzwerk, Serialisierung, etc.)
}
```

## Naechste Schritte

- [Konfigurationsreferenz](/de/docs/configuration/)
- [Architektur-Uebersicht](/de/docs/architecture/)
- [API-Dokumentation](/de/docs/api/)
