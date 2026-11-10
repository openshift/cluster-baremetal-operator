# Test Plan: [GA] Enable IPE (Ironic Prometheus Exporter) on OCP — Asynchronous Metric Collection and Performance

*Structured per IEEE 829 (Standard for Software and System Test Documentation), Test Plan outline.*

---

## 1. Test Plan Identifier

| Field | Value |
|---|---|
| Identifier | `TP-OCPSTRAT-2645-v1.4` |
| Feature | [OCPSTRAT-2645](https://redhat.atlassian.net/browse/OCPSTRAT-2645) — [GA] Enable IPE on OCP: asynchronous metric collection and performance |
| Engineering epic | [METAL-1669](https://redhat.atlassian.net/browse/METAL-1669) |
| Test plan task | [METAL-2013](https://redhat.atlassian.net/browse/METAL-2013) |
| Target release | OpenShift 5.1 |
| Component under test | `ironic-image` (Ironic conductor plus `ironic-prometheus-exporter` container), with integration through `cluster-baremetal-operator` (CBO) and Cluster Monitoring Operator (CMO) |
| QA contact | Jad Haj Yahya |
| Status | Draft |
| Date | 2026-11-09 |

**Revision history**

| Version | Date | Change |
|---|---|---|
| v1.0 | 2026-11-08 | Initial draft |
| v1.1 | 2026-11-09 | Aligned with CBO behavior (effective interval default 60 s, `sensorCollectionInterval` and `disableDefaultPrometheusRules` CR fields, ServiceMonitor without TLS/auth). Added TC-IPE-052/053, e2e automation status (§6.4.1), RK-10/RK-11, and resolved Open Question 3. |
| v1.2 | 2026-11-09 | Review fixes: full label-set comparison for stable labels (§6.2, TC-IPE-033); initial payload before blackholing and alert check moved to TC-IPE-044 (TC-IPE-012); fresh-payload requirement (TC-IPE-052); cycle completion defined independently of notifications (TC-IPE-060); P1 exit criterion split by environment; planned restarts excluded from the failure criterion (§7). |
| v1.3 | 2026-11-10 | Review fixes: label-set comparison uses active series only (§6.2); TC-IPE-052 requires a fresh timestamp and a sensor metric from the same Redfish host; L2 latency profile split into delayed and timed-out nodes (§6.3.5, TC-IPE-060). |
| v1.4 | 2026-11-10 | Review fixes: label-set snapshots must contain sensor series (§6.2, TC-IPE-033); TC-IPE-052 requires a deployed Redfish host and async-path evidence; one fixed L2 delay for both images (§6.3.5). |

---

## 2. Introduction

### 2.1 Purpose

This plan describes the scope, approach, resources, and schedule for verifying the OCP 5.1 increment of the Ironic Prometheus Exporter (IPE) GA feature. **In OCP 5.1 the scope is limited to performance and asynchronous metric collection.** New metrics (chassis sensors, NIC port metrics, PCI devices, threshold-based alert rules) belong to OCP 5.2 and are tracked in [OCPSTRAT-3992](https://redhat.atlassian.net/browse/OCPSTRAT-3992).

The goal: confirm that Ironic's sensor-data collection from Redfish BMCs, now running asynchronously, stays correct, scales to production node counts, puts no unacceptable load on the BMCs or the Ironic conductor, and still delivers metrics through IPE into OpenShift Monitoring and on to RHACM Observability.

### 2.2 Background

- **TP (4.21):** [OCPSTRAT-2378](https://redhat.atlassian.net/browse/OCPSTRAT-2378) shipped IPE as Tech Preview.
- **Initial GA:** [OCPSTRAT-3074](https://redhat.atlassian.net/browse/OCPSTRAT-3074) (Closed) delivered the base GA enablement.
- **This increment (5.1):** replaces the synchronous, conductor-thread-bound Redfish sensor polling with an asynchronous Redfish client and asynchronous sensor collection. It also includes these hardening items:
  - Use `$expand` from sushy (fewer Redfish round-trips).
  - Use only stable identifiers as metric labels.
  - Reduce log noise during collection.
  - Handle deprecated Redfish `Power`/`Thermal` resources (replaced by `PowerSubsystem`/`ThermalSubsystem`).

### 2.3 Current implementation in `ironic-image` (baseline for testing, at [`f371ebe`](https://github.com/openshift/ironic-image/tree/f371ebe3bf94fffb8395ddd2b0aa854fe9b12e6e))

| Item | Location | Behavior |
|---|---|---|
| Exporter entrypoint | [scripts/runironic-exporter](https://github.com/openshift/ironic-image/blob/f371ebe3bf94fffb8395ddd2b0aa854fe9b12e6e/scripts/runironic-exporter) | Forces `SEND_SENSOR_DATA=true` and `PROVISIONING_IP=127.0.0.1`, then runs `gunicorn` with 4 workers serving `ironic_prometheus_exporter.app.wsgi` on `${FLASK_RUN_HOST:-0.0.0.0}:${FLASK_RUN_PORT:-9608}` |
| Feature toggle | [scripts/configure-ironic.sh:46](https://github.com/openshift/ironic-image/blob/f371ebe3bf94fffb8395ddd2b0aa854fe9b12e6e/scripts/configure-ironic.sh#L46) | `SEND_SENSOR_DATA` defaults to `false` |
| Shared notification directory | [scripts/configure-ironic.sh:93](https://github.com/openshift/ironic-image/blob/f371ebe3bf94fffb8395ddd2b0aa854fe9b12e6e/scripts/configure-ironic.sh#L93) | Creates `/shared/ironic_prometheus_exporter` |
| Ironic config | [ironic-config/ironic.conf.j2:243-259](https://github.com/openshift/ironic-image/blob/f371ebe3bf94fffb8395ddd2b0aa854fe9b12e6e/ironic-config/ironic.conf.j2#L243-L259) | When `SEND_SENSOR_DATA=true`: `[oslo_messaging_notifications] driver=prometheus_exporter`, `location=/shared/ironic_prometheus_exporter`, `transport_url=fake://`; `[sensor_data] interval=160` (must not go below 120 s); `[metrics] backend=collector` |
| Config override | [README.md:79-83](https://github.com/openshift/ironic-image/blob/f371ebe3bf94fffb8395ddd2b0aa854fe9b12e6e/README.md#L79-L83) | `OS_<SECTION>__<OPTION>` environment variables override rendered config (for example `OS_SENSOR_DATA__INTERVAL`) |
| Pinned sources | [requirements.cachito](https://github.com/openshift/ironic-image/blob/f371ebe3bf94fffb8395ddd2b0aa854fe9b12e6e/requirements.cachito) | `openshift/openstack-ironic`, `openshift/openstack-ironic-prometheus-exporter`, `openshift/openstack-sushy` forks; `prometheus_client==0.22.1` |

### 2.3.1 Current implementation in `cluster-baremetal-operator` (OCP integration baseline)

On OCP, CBO sets the Ironic environment, so the effective defaults differ from the standalone `ironic-image` defaults above.

| Item | Location | Behavior |
|---|---|---|
| CR API | `api/v1alpha1/provisioning_types.go` (`PrometheusExporter`) | `enabled` (bool); `sensorCollectionInterval` (int, CRD `default: 60`, `minimum: 60`); `disableDefaultPrometheusRules` (bool, default `false`) |
| Ironic env | `provisioning/baremetal_pod.go` (`metal3-ironic` container) | When enabled: `SEND_SENSOR_DATA=true` and `OS_SENSOR_DATA__INTERVAL=<sensorCollectionInterval>`. **The effective default interval on OCP is 60 s, not the 160 s set in `ironic.conf.j2`.** |
| Exporter container | `provisioning/baremetal_pod.go` | `metal3-ironic-prometheus-exporter` container (`/bin/runironic-exporter`), port `metrics`/9608. The metal3 pod uses `hostNetwork: true`, so 9608 listens on the control-plane node IP. |
| ServiceMonitor | `provisioning/state_service.go` | `metal3-ironic-prometheus-exporter`, a single endpoint `port: metrics` with **no scheme, TLS, or bearer-token settings** (plain HTTP scrape). Deleted when disabled. |
| PrometheusRule | `provisioning/state_service.go` | `metal3-defaults` (`baremetal.health`, `baremetal.monitoring`). `BaremetalStaleMetrics`: `(time() - baremetal_last_payload_timestamp_seconds) > 300`, `for: 5m`. Deleted when disabled or when `disableDefaultPrometheusRules=true`. |
| Stale-series behavior | IPE `messaging.py` / `app/exporter.py` (upstream) | IPE writes one file per node (`<node_name>-<event_type>`) and never deletes it; `/metrics` concatenates every file. A node that stops reporting keeps its last `baremetal_last_payload_timestamp_seconds`, so `BaremetalStaleMetrics` fires for it. BMO names Ironic nodes `<namespace>~<bmh name>` (`node_name` label). |

### 2.4 References

| ID | Document |
|---|---|
| R1 | [OCPSTRAT-2645](https://redhat.atlassian.net/browse/OCPSTRAT-2645) — Feature definition (functional and non-functional requirements, hardware BoM) |
| R2 | [RFE-8439](https://redhat.atlassian.net/browse/RFE-8439) — Extend Metal3 server metrics collection |
| R3 | Ironic spec: async metrics collection — https://review.opendev.org/c/openstack/ironic-specs/+/972754 ([METAL-1710](https://redhat.atlassian.net/browse/METAL-1710)) |
| R4 | Async Redfish client — https://review.opendev.org/c/openstack/ironic/+/970450 ([METAL-1965](https://redhat.atlassian.net/browse/METAL-1965)) |
| R5 | Async sensor data collection — [METAL-1966](https://redhat.atlassian.net/browse/METAL-1966) |
| R6 | Log-noise reduction — https://review.opendev.org/c/openstack/ironic/+/976767 ([METAL-1753](https://redhat.atlassian.net/browse/METAL-1753)) |
| R7 | Deprecated Redfish sensor endpoints — https://review.opendev.org/c/openstack/sushy/+/989988 ([METAL-1448](https://redhat.atlassian.net/browse/METAL-1448)) |
| R8 | Stable metric labels — https://bugs.launchpad.net/ironic-prometheus-exporter/+bug/2129602 ([METAL-1657](https://redhat.atlassian.net/browse/METAL-1657)) |
| R9 | `$expand` usage — [METAL-1646](https://redhat.atlassian.net/browse/METAL-1646) |
| R10 | CBO e2e gap — [OCPBUGS-130526](https://redhat.atlassian.net/browse/OCPBUGS-130526), https://github.com/openshift/cluster-baremetal-operator/pull/683 |
| R11 | Polarion cases: OCP-88191, OCP-90681, OCP-90682, OCP-90683, OCP-90726, OCP-91036 (TC-IPE-049), OCP-91037 (TC-IPE-053), OCP-91038 (TC-IPE-044) |
| R12 | Docs epic — [OSDOCS-17339](https://redhat.atlassian.net/browse/OSDOCS-17339) |
| R13 | Scale design doc (linked from R1) |
| R14 | IEEE 829-2008, Standard for Software and System Test Documentation |

---

## 3. Test Items

| # | Item | Version / Source |
|---|---|---|
| TI-1 | `ironic-image` container (OCP build, `Dockerfile.ocp`) | OCP 5.1 payload |
| TI-2 | Ironic conductor with the async Redfish client and async sensor collection | `openshift/openstack-ironic` at the commit pinned in `requirements.cachito` |
| TI-3 | `ironic-prometheus-exporter` (IPE) | `openshift/openstack-ironic-prometheus-exporter` at the pinned commit |
| TI-4 | `sushy` Redfish library (`$expand`, `PowerSubsystem`/`ThermalSubsystem`) | `openshift/openstack-sushy` at the pinned commit |
| TI-5 | `cluster-baremetal-operator`: `Provisioning.spec.prometheusExporter` (`enabled`, `sensorCollectionInterval`, `disableDefaultPrometheusRules`), ServiceMonitor `metal3-ironic-prometheus-exporter`, PrometheusRule `metal3-defaults` | OCP 5.1 payload |
| TI-6 | Integration with CMO (platform Prometheus) and RHACM Observability | OCP 5.1 / supported RHACM release |

---

## 4. Features to Be Tested

### 4.1 Asynchronous sensor collection (primary focus)

| ID | Feature | Trace |
|---|---|---|
| F-1 | Sensor data is collected through the async Redfish client and published as notifications that IPE consumes | METAL-1965, METAL-1966 |
| F-2 | A slow, unresponsive, or erroring BMC does not block or delay collection for other nodes | METAL-1966, R3 |
| F-3 | Async collection does not delay other periodic conductor tasks (power-state sync, provisioning, inspection, cleaning) | R1 NFR Performance |
| F-4 | The collection interval holds and the minimum-interval rule is respected. Effective default: **60 s on OCP** (CBO `sensorCollectionInterval`, API minimum 60 s); 160 s only for standalone `ironic-image`. No BMC is polled more often than every 60 s. The conflict with the upstream ≥120 s guidance is tracked as RK-10. | ironic.conf.j2, CBO §2.3.1 |
| F-5 | Concurrency limits and timeouts for async requests work as configured. Option names come from the R3/R4 implementation. | R3, R4 |
| F-6 | `$expand` reduces the Redfish request count per node per cycle | METAL-1646 |

### 4.2 Performance and scalability

| ID | Feature | Trace |
|---|---|---|
| F-7 | Conductor CPU, memory, thread/greenthread, and file-descriptor use while collecting from N nodes | R1 NFR Performance |
| F-8 | Collection-cycle duration as N grows (baseline sync vs async) | R1 NFR Performance |
| F-9 | Ironic API latency during collection | R1 NFR Performance |
| F-10 | IPE `/metrics` response time and payload size as N grows; gunicorn 4-worker behavior | runironic-exporter |
| F-11 | Disk growth of the shared notification directory (`/shared/ironic_prometheus_exporter`) | configure-ironic.sh |
| F-12 | Load on the BMC per node (requests per cycle, concurrent sessions) | R1 NFR Performance |
| F-13 | Federation of hardware metrics from many managed clusters to the RHACM hub. Volume and cardinality are measured, not full 3500-SNO scale. | R1 NFR Scalability |

### 4.3 Correctness and hardening

| ID | Feature | Trace |
|---|---|---|
| F-14 | Metric labels use only stable identifiers (no per-cycle churn, no new series after a conductor restart) | METAL-1657 |
| F-15 | Sensors are read from `PowerSubsystem`/`ThermalSubsystem`, with fallback to deprecated `Power`/`Thermal` | METAL-1448 |
| F-16 | Logs carry no excessive per-sensor or per-cycle messages at default log level | METAL-1753 |
| F-17 | Metric values match the BMC's Redfish data (temperature, fan, power supply, drive, and other existing metrics) | R1 FR |

### 4.4 Integration and regression of the GA base

| ID | Feature | Trace |
|---|---|---|
| F-18 | Enable and disable through `Provisioning.spec.prometheusExporter.enabled` (Day-1 and Day-2) | OCP-88191 |
| F-19 | ServiceMonitor, PrometheusRule, and metrics port 9608 are created and removed | OCP-90681 |
| F-20 | CMO scrapes `job=metal3-state` | OCP-90682 |
| F-21 | Default `baremetal.health` / `baremetal.monitoring` rule groups load, including `BaremetalStaleMetrics` | OCP-90683 |
| F-22 | Out-of-the-box default configuration, and user customization through the CBO CR field `spec.prometheusExporter.sensorCollectionInterval` (validated, minimum 60 s) and `OS_<SECTION>__<OPTION>` overrides (standalone) | R1 FR |
| F-23 | Metrics can be queried on the RHACM hub Observability service (Grafana/Thanos) | R1 FR |
| F-24 | Behavior with TLS, FIPS, IPv4/IPv6/dual-stack, and upgrade from 5.0 | Regression |
| F-25 | `spec.prometheusExporter.disableDefaultPrometheusRules` removes only the default PrometheusRule; the ServiceMonitor, metrics port, and scraping are unaffected | CBO §2.3.1 |

---

## 5. Features Not to Be Tested

| Item | Reason |
|---|---|
| New chassis sensor metrics (`/redfish/v1/Chassis/*/Sensors`) | OCP 5.2 scope — OCPSTRAT-3992 |
| Network port / transceiver metrics, PCI device metrics | OCP 5.2 scope — OCPSTRAT-3992 |
| Threshold-derived PrometheusRules generated from sensor thresholds | OCP 5.2 scope — OCPSTRAT-3992 |
| Full metric inventory analysis for Dell/HPE | OCP 5.2 scope |
| Smartctl metrics (RFE-5596) | Separate RFE, not committed |
| Non-Redfish drivers (IPMI, iDRAC-WSMAN, iLO legacy) | Async client targets Redfish only. Smoke check only: these must not regress when sensor data is enabled. |
| Full 3500-SNO hub federation scale | Owned by RHACM/Observability scale teams. This plan measures per-cluster contribution (series count, sample rate) for extrapolation. |
| OKD/SCOS image variants | Not a product deliverable for this feature |

---

## 6. Approach

### 6.1 Test levels

| Level | Owner | Description |
|---|---|---|
| Unit | Dev (upstream) | Ironic/sushy/IPE unit tests for the async client, timeouts, error paths, and label generation. QE reviews coverage. |
| Component | QE | Standalone `ironic-image` (podman) with sushy-tools / fake Redfish BMCs. Verifies config rendering, the exporter entrypoint, collection timing, and failure isolation. |
| Pre-merge (METAL-1672) | QE | Run the component suite against PR builds of the openshift forks. |
| Integration / e2e (METAL-1671) | QE | OCP 5.1 cluster with CBO; Ginkgo e2e for CBO/CMO integration (OCP-88191, 90681-90683) plus new async cases. |
| System (hardware) | QE | Real Dell (Prio 1) and HPE (Prio 2) servers from the BoM. |
| Performance / scale | QE + Perf | Virtual BMCs (sushy-tools emulator with delay injection) at 50/100/250/500 nodes per conductor. Real-hardware spot check. |
| CI (METAL-1670) | QE | Add the e2e cases to periodic Prow jobs for the bare-metal (virtual media / IPI) lanes. |

### 6.2 Techniques

- **Baseline comparison:** run the same performance scenarios against a 5.0 (synchronous) image and a 5.1 (async) image. Async must not be worse on any measured axis and must improve cycle duration under BMC latency.
- **Fault injection:** a fake Redfish proxy (sushy-tools emulator or mitmproxy) adds delay, timeouts, HTTP 5xx/401/404, malformed JSON, and connection resets for a subset of nodes.
- **Equivalence partitioning** for the interval (`<60` rejected by the API, `=60` OCP default, `120`, `160` standalone default, large) and for concurrency/timeout settings.
- **Label-set comparison:** before and after restarts and BMC firmware resets, record the full label set of every active `{job="metal3-state"}` series and diff the two sets. Take each snapshot from an instant query of `{job="metal3-state"}` or from the raw IPE `/metrics` output. Do not use `/api/v1/series`: it also returns inactive series inside its time range, so old and new label sets can both appear and hide a change. Both snapshots must contain Redfish sensor series (`baremetal_*_status`, `baremetal_temp_*_celsius`) for every tested host; a snapshot with only `up` and scrape metrics is invalid. Any added, removed, or changed label value is churn. `count by (__name__)` is only a summary, because a label can change while the count stays the same.
- **Black-box** checks against Prometheus/Thanos APIs; **white-box** checks of rendered `ironic.conf` and conductor logs.

### 6.3 Test cases

Priority: **P1** = release-blocking, **P2** = high, **P3** = nice to have.

#### 6.3.1 Configuration and enablement (component)

| TC ID | Title | Steps (summary) | Expected result | Pri | Trace |
|---|---|---|---|---|---|
| TC-IPE-001 | Sensor collection is disabled by default | Start `ironic` container without `SEND_SENSOR_DATA`; inspect `/etc/ironic/ironic.conf` | No `[oslo_messaging_notifications]`, `[sensor_data]`, or `[metrics]` sections; no notifications written to `/shared/ironic_prometheus_exporter` | P1 | F-22 |
| TC-IPE-002 | Enabling sensor data renders the correct config | `SEND_SENSOR_DATA=true` | `driver=prometheus_exporter`, `location=/shared/ironic_prometheus_exporter`, `transport_url=fake://`, `send_sensor_data=true`, `interval=160`, `backend=collector` | P1 | F-22 |
| TC-IPE-003 | Exporter entrypoint starts and serves `/metrics` | Run `runironic-exporter` sharing `/shared` with the conductor; `curl :9608/metrics` | HTTP 200, Prometheus text format, 4 gunicorn workers, no interface-detection failure (dummy `PROVISIONING_IP`) | P1 | F-10 |
| TC-IPE-004 | Custom host/port | Set `FLASK_RUN_HOST`/`FLASK_RUN_PORT` | Exporter binds to the given address | P3 | F-22 |
| TC-IPE-005 | `OS_` override takes precedence | `SEND_SENSOR_DATA=true` plus `OS_SENSOR_DATA__INTERVAL=300` | Effective interval is 300 s (confirmed in log timestamps) | P2 | F-22 |
| TC-IPE-006 | Async tuning options can be overridden | Set the async concurrency/timeout options from R3/R4 through `OS_` vars | Options take effect (observable concurrency and timeout behavior) | P2 | F-5 |

#### 6.3.2 Asynchronous collection behavior (component, virtual BMCs)

| TC ID | Title | Steps (summary) | Expected result | Pri | Trace |
|---|---|---|---|---|---|
| TC-IPE-010 | Basic async collection | Enroll 10 Redfish nodes (sushy-tools); enable sensor data; wait 2 intervals | Metrics for all 10 nodes appear; one notification batch per interval; logs/stack show async client code path | P1 | F-1 |
| TC-IPE-011 | One slow BMC does not delay the others | 20 nodes; node A has 60 s response delay | All other nodes report within the normal cycle time; node A reports late or times out; total cycle time ≈ max(timeout, fast-node time), not the sum | P1 | F-2 |
| TC-IPE-012 | Unreachable BMC | Wait until IPE `/metrics` has a `baremetal_last_payload_timestamp_seconds` sample for node A; then blackhole node A's BMC IP and wait 3 intervals | Timeout after the configured value; error logged once per cycle (not per sensor); other nodes' timestamps keep advancing; node A's timestamp stops advancing while IPE keeps serving its last value. The component environment has no Prometheus or CBO rule, so the `BaremetalStaleMetrics` alert is verified in-cluster by TC-IPE-044 | P1 | F-2, F-16 |
| TC-IPE-013 | BMC returns HTTP 5xx / 401 / malformed JSON | Inject errors for a subset of nodes | No conductor crash or traceback storm; failing nodes skipped; healthy nodes reported | P1 | F-2 |
| TC-IPE-014 | Connection reset mid-collection | Reset TCP during `$expand` response | Request fails cleanly; no leaked sessions or file descriptors (fd count stable across 20 cycles) | P2 | F-2, F-7 |
| TC-IPE-015 | Power-state sync is not delayed | 100 nodes, 30% with 30 s BMC latency | `[conductor]sync_power_state_interval` (60 s) keeps running on time; no "power state sync skipped/overrun" warnings | P1 | F-3 |
| TC-IPE-016 | Provisioning during collection | Deploy/clean 5 nodes while collection runs on 100 | Provisioning succeeds; deploy time within ±10% of baseline with collection off | P1 | F-3 |
| TC-IPE-017 | Interval holds | Default interval (60 s as set by CBO on OCP; 160 s standalone); measure 10 cycles | Cycle start drift ≤ 5 s; no overlapping cycles for a node | P1 | F-4 |
| TC-IPE-018 | Overlapping cycle protection | Interval 60 s (CBO minimum) and 120 s; BMC latency forces cycle > interval | A node is not polled again while its previous request is still in flight (no request pile-up) | P1 | F-4, F-12 |
| TC-IPE-019 | Concurrency limit respected | Set concurrency limit to X with 200 nodes | Concurrent BMC connections ≤ X (measured at the emulator) | P2 | F-5 |
| TC-IPE-020 | `$expand` reduces Redfish calls | Count requests per node per cycle on an `$expand`-capable emulator vs. an emulator without `$expand` | Fewer requests with `$expand`; correct fallback when `$expand` is not advertised in `ProtocolFeaturesSupported` | P2 | F-6 |
| TC-IPE-021 | Node lifecycle states | Nodes in `enroll`, `manageable`, `available`, `active`, maintenance mode | Collection only covers nodes in the states Ironic defines for sensor collection (maintenance nodes excluded unless configured otherwise) | P2 | F-1 |
| TC-IPE-022 | Node add/delete during collection | Enroll and delete nodes mid-cycle | No errors for deleted nodes beyond one cycle; new nodes appear within ≤ 2 intervals | P2 | F-1 |
| TC-IPE-023 | Conductor restart | Restart the `ironic` container during a cycle | Collection resumes; no duplicate series; IPE serves the last known values until refreshed | P2 | F-14 |
| TC-IPE-024 | Mixed drivers | Redfish + IPMI nodes with sensor data enabled | IPMI nodes keep their current behavior; no errors from the async path | P3 | F-1 |

#### 6.3.3 Metrics correctness and hardening

| TC ID | Title | Steps (summary) | Expected result | Pri | Trace |
|---|---|---|---|---|---|
| TC-IPE-030 | Values match the BMC | For each BoM server, compare IPE metric values with direct Redfish GETs (temperature, fan, PSU, drive health) | Values match within one collection interval; units correct | P1 | F-17 |
| TC-IPE-031 | `ThermalSubsystem`/`PowerSubsystem` used | Dell iDRAC firmware with deprecated `Thermal`/`Power` | Metrics come from the new subsystems; no deprecation errors; values equal pre-change | P1 | F-15 |
| TC-IPE-032 | Fallback to legacy `Thermal`/`Power` | Emulator/firmware exposing only legacy resources | Metrics still produced | P2 | F-15 |
| TC-IPE-033 | Stable labels | Confirm sensor series exist for every tested host; record the full label set of every series (§6.2); restart conductor, restart IPE, reboot the BMC, wait 5 cycles; record again | Label sets identical: no series added, removed, or relabeled; no unique/per-cycle IDs in labels | P1 | F-14 |
| TC-IPE-034 | No alert flapping from label churn | Run 24 h on hardware | Zero alerts resolve and re-fire because a label changed | P2 | F-14 |
| TC-IPE-035 | Log noise | Default log level, 100 nodes, 1 h | No per-sensor INFO/WARNING lines; per-cycle summary only; failures logged once per node per cycle; log volume ≤ the agreed threshold (define with Dev; baseline vs. 5.0) | P2 | F-16 |
| TC-IPE-036 | Notification directory growth | 100 nodes, 24 h | `/shared/ironic_prometheus_exporter` size bounded (old files rotated/overwritten); no inode exhaustion | P2 | F-11 |

#### 6.3.4 OpenShift integration (e2e)

| TC ID | Title | Steps (summary) | Expected result | Pri | Trace |
|---|---|---|---|---|---|
| TC-IPE-040 (OCP-88191) | Enable/disable through Provisioning CR | Patch `spec.prometheusExporter.enabled=true`/`false` | `metal3` pod gains/loses the `metal3-ironic-prometheus-exporter` container; `metal3-state:9608/metrics` serves data when enabled and is unreachable when disabled | P1 | F-18 |
| TC-IPE-041 (OCP-90681) | ServiceMonitor, PrometheusRule, and port | Enable | `ServiceMonitor/metal3-ironic-prometheus-exporter`, `PrometheusRule/metal3-defaults`, and service port 9608 exist; removed on disable | P1 | F-19 |
| TC-IPE-042 (OCP-90682) | CMO scrape | Query `up{job="metal3-state"}` in platform Prometheus | `1`; a `baremetal_last_payload_timestamp_seconds` series (matched by `node_name`) exists for every deployed (provisioned or externally provisioned) BMH with a Redfish/iDRAC BMC address | P1 | F-20 |
| TC-IPE-043 (OCP-90683) | Default rule groups | List rules in Prometheus | `baremetal.health` and `baremetal.monitoring` groups loaded, including `BaremetalStaleMetrics` | P1 | F-21 |
| TC-IPE-044 (OCP-91038) | Stale-metrics alert in-cluster | Pick a provisioned Redfish worker BMH. Stop its collection by setting the `baremetalhost.metal3.io/detached` annotation (Ironic node removed; host keeps running). Wait up to 20 min. Remove the annotation. | `BaremetalStaleMetrics` fires for that host's `node_name` only (no newly firing alert for attached hosts); after re-attach the host returns to `OK`/`provisioned` and the alert resolves within 10 min | P1 | F-21, F-2 |
| TC-IPE-045 | Day-1 enablement | Install with exporter enabled through install-time manifests | Metrics available after install with no manual steps | P1 | F-18 |
| TC-IPE-046 (in OCP-88191) | Day-2 enablement does not disrupt BMHs | Record each BMH's provisioning state, operationalStatus, and `operationHistory.inspect.start`; enable on the running cluster | Every BMH returns to its recorded state and status within 10 min of the metal3 rollout; `inspect.start` unchanged (no re-inspection) | P1 | F-18 |
| TC-IPE-047 | Upgrade 5.0 → 5.1 with exporter enabled | Upgrade the cluster | Exporter stays enabled; async collection active; series continuous (no label change, F-14) | P1 | F-24 |
| TC-IPE-048 | RHACM hub federation | Managed cluster with Observability addon; add the IPE metrics to the allowlist (default or custom, as documented) | Hardware metrics can be queried in hub Grafana/Thanos with the `cluster` label; dashboards render | P1 | F-23 |
| TC-IPE-049 (OCP-91036) | Customization of the interval | 1) Read `spec.prometheusExporter.sensorCollectionInterval`. 2) Patch it to `59`. 3) Patch it to `120` (or `180` if already 120). 4) Delete the metal3 pod. | 1) `OS_SENSOR_DATA__INTERVAL` in `metal3-ironic` equals the spec value (60 if unset). 2) API rejects the patch (minimum 60); spec unchanged. 3) metal3 pod rolls out; env equals the new value. 4) Value persists after the pod restart; `baremetal` ClusterOperator Available | P2 | F-22, F-4 |
| TC-IPE-050 | TLS / FIPS / IP families | Run TC-IPE-040..043 on FIPS-enabled, IPv6-only, and dual-stack clusters | All pass | P2 | F-24 |
| TC-IPE-051 | Exporter security | Inspect the ServiceMonitor endpoint and reach 9608 from a non-monitoring pod and from the node network | Scrape uses CMO-managed TLS/auth; the endpoint does not serve unauthenticated clients; no BMC credentials in metrics or logs. **Known gap:** the ServiceMonitor currently has no TLS/auth and the metal3 pod uses `hostNetwork`, so 9608 serves plain HTTP on the node IP. This case is expected to fail until RK-11 is resolved. | P1 | F-19 |
| TC-IPE-052 (OCP-90726) | In-cluster BMC sensor collection | Prerequisite: at least one deployed BMH with a Redfish/iDRAC BMC address. Skip otherwise; GA sign-off requires a run that meets it. With IPE enabled: check `SEND_SENSOR_DATA` and `send_sensor_data` in `metal3-ironic`; record each deployed Redfish/iDRAC host's `baremetal_last_payload_timestamp_seconds` from `/metrics` (if present); read the conductor logs; wait up to 2 intervals and read `/metrics` again | `SEND_SENSOR_DATA=true`; during the observation window the conductor logs show the async path: `queued for async sensor data collection` for the tested host (where the message names the node), or a `Completed sending sensor data` summary with a non-zero async node count. No `Failed to get sensor data` appears. A summary that reports only sync nodes does not pass, because the synchronous collector also logs `Completed sending sensor data` and can produce fresh Redfish metrics. Confirm the exact log messages against the pinned Ironic commit (T1). For at least one deployed Redfish/iDRAC host, matched by `node_name`, both of these hold: its timestamp is newer than the recorded value or appears for the first time, and `/metrics` has at least one Redfish sensor metric for it (`baremetal_temperature_status`, `baremetal_power_status`, `baremetal_fan_status`, `baremetal_drive_status`, or `baremetal_temp_<context>_celsius`). None of the following pass on their own: a fresh timestamp without sensor metrics (IPE writes the timestamp even when the parser finds no sensors), a timestamp from an IPMI or other non-Redfish host, a queued attempt, or a retained, unchanged series (IPE keeps serving files from earlier notifications). `ironic_*` process timings alone do not pass either | P1 | F-1, F-17 |
| TC-IPE-053 (OCP-91037) | Disable default rules only | Patch `disableDefaultPrometheusRules=true`, then `false` | `true`: `PrometheusRule/metal3-defaults` deleted and its groups unloaded from CMO Prometheus; ServiceMonitor and port 9608 remain; `up{job="metal3-state"}` stays `1`; metal3 pod not restarted. `false`: rule recreated and groups reloaded | P2 | F-25 |

#### 6.3.5 Performance and scale

Run each scenario with the 5.0 (sync) image and the 5.1 (async) image. BMC latency profiles: **L0** = 0.2 s; **L1** = 2 s; **L2** = 5% of nodes **delayed**, plus 5% of nodes that never respond and **time out**. Use one fixed delay for both the 5.0 and 5.1 runs: 30 s, or half the lower of the two images' per-request timeouts if that is shorter, so both images face the same workload. Record which nodes are in each group.

| TC ID | Title | Load | Metrics captured | Acceptance | Pri | Trace |
|---|---|---|---|---|---|---|
| TC-IPE-060 | Cycle duration vs node count | 50/100/250/500 nodes × L0/L1/L2 | Cycle duration: time from cycle start until every node's request has either completed or reached the configured timeout, taken from conductor logs and the emulator request log. A timed-out node may write no notification, so the last notification does not mark the end of the cycle. Also recorded: notification latency per successful node, and the number of nodes that failed or timed out | Async ≤ sync at L0; async ≥ 50% faster at L1/L2 at 250+ nodes (final threshold agreed with Dev); cycle duration < interval at 500 nodes/L1. At L2, the failed/timed-out nodes are exactly the nodes in the time-out group; every delayed node succeeds and writes a notification | P1 | F-8 |
| TC-IPE-061 | Conductor resource overhead | Same matrix | CPU (avg/p95), RSS, greenthreads/threads, open fds, DB connections | Overhead vs. collection off ≤ documented budget (required by R1 NFR); no monotonic growth over 24 h | P1 | F-7 |
| TC-IPE-062 | API latency under collection | 250 nodes/L2; run `baremetal node list/show` loop | p50/p95/p99 API latency | p95 within +20% of collection-off baseline | P1 | F-9 |
| TC-IPE-063 | Exporter scaling | 500 nodes | `/metrics` response time, payload size, series count, gunicorn worker CPU/RSS | Response < scrape timeout (default 10 s) with margin ≥ 50%; series/node recorded for capacity docs | P1 | F-10 |
| TC-IPE-064 | BMC load | Per node, 1 h | Requests/cycle, peak concurrent sessions per BMC | ≤ sync baseline; ≤ 1 concurrent session per BMC from sensor collection | P1 | F-12 |
| TC-IPE-065 | Soak | 250 nodes/L1, 72 h | All the above plus memory trend | No leaks (RSS slope ≈ 0), no stuck cycles, no OOMKill/restarts | P1 | F-7 |
| TC-IPE-066 | Real-hardware performance | Max available BoM servers (Dell XR8620t/XR8720t, HPE DL110 Gen11/12) | Cycle duration, BMC CPU/response (from BMC logs if available) | No BMC-side errors/throttling; matches virtual-BMC trends | P1 | F-8, F-12 |
| TC-IPE-067 | Federation contribution per cluster | SNO + compact cluster | Series and samples/s sent to the hub per node and per cluster | Recorded and handed to RHACM scale team for 3500-SNO extrapolation | P2 | F-13 |
| TC-IPE-068 | SNO footprint | SNO with exporter enabled | Added CPU/mem on SNO control plane | Within SNO platform budget (documented) | P1 | F-7 |

### 6.4 Automation strategy

- **Ginkgo e2e** (CBO repo `tests-extension/openshift/test/e2e/ironic_prometheus_exporter.go`): TC-IPE-040…047 and 049–053. CBO PR #683 covers 041–043. See §6.4.1 for per-case status.
- **Component suite** (podman + sushy-tools, scripted in bash/pytest): TC-IPE-001…036, running pre-merge on fork PRs.
- **Perf harness:** sushy-tools emulator farm, a latency/fault proxy, and Prometheus scraping the conductor host (node-exporter) and the exporter. Results stored per run for baseline comparison.
- **Manual:** hardware value comparison (TC-IPE-030/031), RHACM dashboards (TC-IPE-048), soak review.
- Test cases are registered in Polarion and linked to METAL-1671.

#### 6.4.1 E2E automation status

| TC | Ginkgo spec | Status | Notes |
|---|---|---|---|
| TC-IPE-040 | OCP-88191 | Automated | Checks the exporter container in the pod spec on enable and disable |
| TC-IPE-041 | OCP-90681 | Automated | Removal is asserted in the ordered container's `AfterAll` |
| TC-IPE-042 | OCP-90682 | Automated | Per-host check covers deployed Redfish/iDRAC BMHs; skipped when none exist |
| TC-IPE-043 | OCP-90683 | Automated | |
| TC-IPE-044 | OCP-91038 | Automated | `Longduration`, `[Disruptive]`; detaches workers only, never the control plane; last spec in the ordered container |
| TC-IPE-045 | — | Not automated | Needs an install-time Prow job with the exporter enabled (METAL-1670) |
| TC-IPE-046 | OCP-88191 | Automated | Folded into the Day-2 enable step |
| TC-IPE-047 | — | Not automated | Needs pre-upgrade/post-upgrade hooks in an upgrade job (METAL-1670) |
| TC-IPE-049 | OCP-91036 | Automated | Interval only (the CR exposes no other collection setting) |
| TC-IPE-050 | existing specs | CI config | Run 040–043 in FIPS, IPv6, and dual-stack lanes (METAL-1670) |
| TC-IPE-051 | — | Blocked | Expected to fail until RK-11 is decided; automate once the target behavior is agreed |
| TC-IPE-052 | OCP-90726 | Partially automated | The spec checks that the series is present. Not yet implemented (follow-up in #683): skip unless a deployed Redfish/iDRAC BMH exists (it currently skips only when no BMH is deployed), require the timestamp to advance with a sensor metric for the same Redfish host, and require async-path evidence (it also accepts a sync-only summary) |
| TC-IPE-053 | OCP-91037 | Automated | |

---

## 7. Item Pass/Fail Criteria

**Feature passes when:**

1. 100% of P1 test cases pass in the environment assigned to them:
   - **Virtual BMCs:** component and async cases (TC-IPE-001…024) and scale cases (TC-IPE-060…065). The hardware lab has only 3 servers per Dell model, so scale cases do not run on hardware.
   - **Prio 1 hardware (Dell XR8620t, XR8720t):** TC-IPE-030, 031, 033, and 066, plus the P1 OpenShift integration cases (TC-IPE-040…052) on a cluster built from that hardware. TC-IPE-068 runs on an XR8620t SNO.
   - The integration cases also run in the virtual-media CI lanes (METAL-1670). A CI pass does not replace the hardware run.
2. ≥ 95% of P2 test cases pass. Every failure has an accepted bug with a workaround or a deferral.
3. Performance acceptance (TC-IPE-060…068) is met, and the documented overhead and default intervals are published (R1 NFR).
4. No open Blocker/Critical bugs against TI-1…TI-5.
5. HPE (Prio 2) results are reported. Failures do not block GA unless they are crash/data-corruption class.

**An individual test case fails if** the observed result differs from the expected result, the conductor or exporter crashes or restarts without the test's steps causing it, or BMC credentials or other sensitive data appear in logs or metrics. Restarts that are part of a case's steps (for example TC-IPE-023, TC-IPE-040, TC-IPE-049) are judged against that case's expected result.

---

## 8. Suspension Criteria and Resumption Requirements

| Suspend when | Resume when |
|---|---|
| The async collection patches (R4/R5) are not merged into the openshift forks or not pinned in `requirements.cachito` | Pinned commit is available in a nightly 5.1 payload |
| The conductor crashes or deadlocks repeatedly in basic collection (TC-IPE-010) | A fix is merged and TC-IPE-010 passes |
| The BoM hardware lab is unavailable for more than 5 working days | Lab restored; virtual-BMC testing continues in the meantime |
| The CBO exporter enablement is broken in the payload | A payload with a fix is available |
| Perf environment instability (> 10% run-to-run variance at the baseline) | Environment stabilized and baseline re-captured |

---

## 9. Test Deliverables

- This test plan (`TP-OCPSTRAT-2645-v1.4`)
- Test case specifications in Polarion (linked to METAL-1671 / METAL-2013)
- Automated test code: Ginkgo e2e PRs, the component test suite, and the perf harness scripts
- Prow periodic job configuration (METAL-1670)
- Test execution logs and the Polarion test run
- Performance report: sync vs async comparison, overhead, recommended default interval and tuning guidance (input to OSDOCS-17339)
- Defect reports (OCPBUGS)
- Test summary report (IEEE 829 Test Summary Report) at feature sign-off

---

## 10. Testing Tasks

| # | Task | Depends on | Jira |
|---|---|---|---|
| T1 | Review the async spec (R3) and implementation (R4/R5); record config option names and defaults | — | METAL-2013 |
| T2 | Finalize the test plan; get approvals | T1 | METAL-2013 |
| T3 | Build the virtual-BMC and fault-injection environment | — | METAL-1672 |
| T4 | Implement and run the component suite on fork PRs | T3 | METAL-1672 |
| T5 | Implement the Ginkgo e2e cases; land CBO PR #683 coverage | T2 | METAL-1671, OCPBUGS-130526 |
| T6 | Add e2e to Prow periodics | T5 | METAL-1670 |
| T7 | Capture the 5.0 sync performance baseline | T3 | — |
| T8 | Run 5.1 async performance and scale runs; write the perf report | T7, async merged | — |
| T9 | Hardware validation on the Dell/HPE BoM | Lab access | — |
| T10 | RHACM federation validation | 5.1 cluster + RHACM hub | — |
| T11 | Upgrade testing 5.0 → 5.1 | 5.1 payload | — |
| T12 | Review documentation | OSDOCS-17339 draft | OSDOCS-17339 |
| T13 | Test summary report and sign-off | All | METAL-2013 |

---

## 11. Environmental Needs

### 11.1 Hardware (from the R1 BoM)

| Vendor | Model | Priority | Qty (min) | Use |
|---|---|---|---|---|
| Dell | XR8620t | Prio 1 | 3 | Primary qualification, SNO + compact |
| Dell | XR8720t | Prio 1 | 3 | Primary qualification |
| HPE | DL110 Gen 11 | Prio 2 | 1 | Secondary qualification |
| HPE | DL110 Gen 12 | Prio 2 | 1 | Secondary qualification |

Each server needs a recent BMC firmware (iDRAC 9/10, iLO 6/7). For TC-IPE-031/032, it also needs firmware revisions that expose both the legacy and the new Thermal/Power resources.

### 11.2 Virtual / scale

- Hypervisor host(s) able to run 500 sushy-tools emulated Redfish BMCs (fake-system or libvirt-backed), plus a latency/fault-injection proxy.
- A standalone `ironic-image` deployment (podman) on a host comparable to an OCP control-plane node (8 vCPU / 32 GiB).
- An OCP 5.1 dev-scripts / IPI bare-metal environment with virtual-media BMCs.

### 11.3 Software

- OCP 5.1 nightly payloads; an OCP 5.0 GA image for the baseline.
- RHACM (version supported with OCP 5.1) with the Observability addon and S3-compatible object storage.
- Prometheus/Grafana for perf telemetry; `promtool`; `oc`, `baremetal` CLI; `curl`/`jq`.
- Polarion, Prow CI access.

---

## 12. Responsibilities

| Role | Person | Responsibility |
|---|---|---|
| QA contact / test lead | Jad Haj Yahya | Plan, test design, execution, reporting, sign-off recommendation |
| Feature assignee | Eduardo Terrell Ferrari Otubo | Feature delivery, triage coordination |
| Architect | Iury Gregory Melo Ferreira | Async design clarification, perf acceptance thresholds |
| Upstream Ironic dev | Dmitry Tantsur (spec), Ironic team | Defect fixes in ironic/sushy/IPE |
| Product manager | Michal Zasepa | Scope decisions, acceptance of deferrals |
| Docs | Avani Bhatt | Documentation of defaults, overhead, customization (OSDOCS-17339) |
| Product Ops Eng | Derrick Ornelas | Supportability review |
| RHACM Observability | TBD | Hub-side federation and scale validation |

---

## 13. Staffing and Training Needs

- 1 QE engineer (full-time during execution), plus a perf/scale engineer for about 2 weeks of runs.
- Training and familiarization:
  - The async collection design (R3).
  - sushy-tools emulator and fault injection.
  - Redfish `$expand` and `ThermalSubsystem`/`PowerSubsystem` schemas.
  - RHACM Observability allowlist configuration.
  - PromQL for cardinality analysis.

---

## 14. Schedule

Dates follow the OCP 5.1 release milestones. Adjust when the 5.1 dates are final.

| Milestone | Entry condition | Activities |
|---|---|---|
| M1 — Plan approved | T1 done | T2 |
| M2 — Environment ready | Lab + virtual BMC farm | T3, T7 |
| M3 — Pre-merge testing | Async PRs in review (METAL-1965/1966 in Code Review) | T4 |
| M4 — Feature complete (5.1 FF) | Async merged + pinned in `requirements.cachito` | T5, T6, T8, T9 |
| M5 — Integration / upgrade | 5.1 nightly with CBO changes | T10, T11 |
| M6 — Code freeze | — | Regression of P1 suite, T12 |
| M7 — GA sign-off | All exit criteria met | T13 |

---

## 15. Risks and Contingencies

| # | Risk | Likelihood | Impact | Mitigation / contingency |
|---|---|---|---|---|
| RK-1 | Async patches (R4/R5) land late in 5.1 | Med | High | Run pre-merge on PR builds; escalate to PM for a scope decision at FF |
| RK-2 | Perf overhead budget not defined in R1 | High | High | Agree on numeric thresholds with the architect before M4; mark TC-IPE-060/061 thresholds provisional until then |
| RK-3 | Limited BoM hardware quantity | Med | Med | Use virtual BMCs for scale; use hardware for correctness and BMC-side behavior |
| RK-4 | BMC firmware differences cause inconsistent sensor sets | Med | Med | Record firmware versions in results; test both deprecated and new resource paths |
| RK-5 | Async concurrency overloads BMCs (iDRAC session limits) | Med | High | TC-IPE-019/064 cap concurrency; verify ≤ 1 session per BMC; tune defaults |
| RK-6 | RHACM hub cardinality grows with many SNOs | Med | Med | Provide per-cluster series counts (TC-IPE-067); recommend an allowlist subset |
| RK-7 | Priority conflict in R1: the text names Dell and HPE both as Prio 1, the BoM lists HPE as Prio 2 | — | Low | This plan follows the BoM (HPE = Prio 2); PM to confirm |
| RK-8 | RHACM federation might be in 5.1 scope or might only be regression from OCPSTRAT-3074 | — | Med | Treated as P1 regression (TC-IPE-048); PM to confirm |
| RK-9 | Eventlet-to-threading/asyncio changes in Ironic interact with gunicorn/IPE | Low | Med | Covered by component soak (TC-IPE-065) |
| RK-10 | CBO defaults `sensorCollectionInterval` to 60 s and accepts 60 s, while the `ironic-image` guidance says the interval must not go below 120 s. At 60 s, BMC load and cycle overlap risk are higher than the 160 s baseline assumes. | High | High | Architect to decide whether to keep the 60 s default/minimum or raise it to ≥120 s (Open Question 6). Run TC-IPE-018/060/064 at 60 s as well as 120 s. Docs (OSDOCS-17339) must state the effective OCP default. |
| RK-11 | The IPE metrics endpoint is unauthenticated plain HTTP on the control-plane host network (port 9608), and the ServiceMonitor defines no TLS/auth | High | Med | Raise an OCPBUGS/RFE to front 9608 with kube-rbac-proxy (or equivalent) and add TLS/bearer-token settings to the ServiceMonitor. Until then TC-IPE-051 is blocked; PM to accept or defer (Open Question 7). |

---

## 16. Approvals

| Role | Name | Signature | Date |
|---|---|---|---|
| QE Lead | Jad Haj Yahya | | |
| Engineering (Assignee) | Eduardo Terrell Ferrari Otubo | | |
| Architect | Iury Gregory Melo Ferreira | | |
| Product Manager | Michal Zasepa | | |

---

### Appendix A — Requirements Traceability Matrix

| Requirement (R1) | Test cases |
|---|---|
| FR: IPE collects metrics from Redfish BMCs | TC-IPE-010, 030, 042, 052 |
| FR: Dell/HPE validation (Prio 1/2) | TC-IPE-030, 031, 066 |
| FR: Metrics can be queried from the RHACM hub | TC-IPE-048, 067 |
| FR: Default config works out of the box | TC-IPE-001, 002, 040, 045, 052 |
| FR: User customization of collection | TC-IPE-005, 006, 049, 053 |
| NFR: Performance (minimal overhead, documented defaults) | TC-IPE-015…019, 060…066, 068 |
| NFR: Scalability (hundreds/thousands of nodes, many clusters) | TC-IPE-060, 063, 067 |
| NFR: Usability (RHACM UI) | TC-IPE-048 |
| NFR: Maintainability / extensibility | Code review of R4/R5; TC-IPE-020, 032 (fallback paths) |
| 5.1 scope: asynchronous collection | TC-IPE-010…024 |
| METAL-1657 stable labels | TC-IPE-033, 034, 047 |
| METAL-1753 log noise | TC-IPE-012, 035 |
| METAL-1448 deprecated endpoints | TC-IPE-031, 032 |
| METAL-1646 `$expand` | TC-IPE-020 |
| OCPBUGS-130526 e2e gap | TC-IPE-041, 042, 043 |
| CBO `sensorCollectionInterval` / `disableDefaultPrometheusRules` | TC-IPE-049, 053 |
| Exporter endpoint security | TC-IPE-051 (blocked, RK-11) |

### Appendix B — Open Questions (resolve before M4)

1. What are the exact config option names and defaults for async concurrency and per-request timeout? (from R3/R4)
2. What is the numeric overhead budget (CPU/mem on the conductor, per-node BMC requests), and what is the recommended default interval for GA documentation?
3. ~~Is the customization mechanism exposed through the CBO `Provisioning` CR, or only through `OS_` overrides?~~ **Resolved:** the CR exposes `sensorCollectionInterval` (min 60, default 60) and `disableDefaultPrometheusRules`. Other collection settings are not exposed through the CR.
4. Are the hardware metrics in the default RHACM Observability allowlist, or must users add a custom allowlist?
5. Should nodes in maintenance mode be excluded from collection?
6. Should the OCP default and minimum `sensorCollectionInterval` stay at 60 s, or be raised to match the ≥120 s `ironic-image` guidance and the 160 s rendered default? (RK-10)
7. Is the unauthenticated, host-network IPE endpoint acceptable for GA, or must the ServiceMonitor scrape go through TLS/auth? (RK-11)
