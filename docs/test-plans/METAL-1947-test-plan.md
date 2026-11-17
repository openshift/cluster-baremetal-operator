# Level Test Plan: Virtual Media Server TLS

| | |
|---|---|
| Document identifier | CBO-LTP-METAL-1947 |
| Standard | IEEE 829-2008, Level Test Plan |
| Test level | System test of the Cluster Baremetal Operator on OpenShift |
| Version | 1.1 |
| Date | 2026-11-17 |
| Epic | [METAL-1947](https://redhat.atlassian.net/browse/METAL-1947) |
| Tracking ticket | [METAL-2009](https://redhat.atlassian.net/browse/METAL-2009) |
| Parent initiative | [OCPSTRAT-3718](https://redhat.atlassian.net/browse/OCPSTRAT-3718) |
| Target release | openshift-5.1 |

This plan follows the engineering epic. [METAL-1947](https://redhat.atlassian.net/browse/METAL-1947) configures the HTTPS server that serves virtual-media images to BMCs. It does not configure Ironic's Redfish client. The `groups` field on the same object is in scope because the implementation carries the virtual-media listener portion of [METAL-1946](https://redhat.atlassian.net/browse/METAL-1946).

## 1. Introduction

### 1.1 Scope

This level covers `provisioning.metal3.io/v1alpha1` `Provisioning.spec.virtualMediaServerTLS` on the singleton `provisioning-configuration` in `openshift-machine-api`, and the effect of that field on the `metal3-httpd` container.

The object has three fields:

- `minTLSVersion` (required): `VersionTLS10`, `VersionTLS11`, `VersionTLS12`, or `VersionTLS13`. The listener accepts that version and every later version through TLS 1.3. There is no maximum-version field.
- `ciphers` (optional): OpenSSL cipher names. Names that start with `TLS_` configure TLS 1.3. Other names configure TLS 1.0 through 1.2. When the list is omitted, the cluster profile or the image defaults remain. When a supplied list omits one family, that family stays enabled and uses OpenSSL defaults.
- `groups` (optional), in preference order: `X25519`, `secp256r1`, `secp384r1`, `secp521r1`, `X25519MLKEM768`, `SecP256r1MLKEM768`, `SecP384r1MLKEM1024`. When omitted, the cluster profile or the image defaults remain.

When the object is unset, both the Ironic API listener and the virtual-media listener follow the cluster TLS profile. When it is set, only these virtual-media variables change:

| Setting | Virtual-media variable | Variable that must stay unchanged |
|---|---|---|
| Minimum version | `IRONIC_VMEDIA_SSL_PROTOCOL` | `IRONIC_SSL_PROTOCOL` |
| TLS 1.0–1.2 ciphers | `IRONIC_VMEDIA_TLS_12_CIPHERS` | `IRONIC_TLS_12_CIPHERS` |
| TLS 1.3 ciphers | `IRONIC_VMEDIA_TLS_13_CIPHERS` | `IRONIC_TLS_13_CIPHERS` |
| Groups | `IRONIC_VMEDIA_CURVES` | `IRONIC_TLS_CURVES` |

The protocol values are:

| `minTLSVersion` | `IRONIC_VMEDIA_SSL_PROTOCOL` |
|---|---|
| `VersionTLS10` | `-ALL +TLSv1 +TLSv1.1 +TLSv1.2 +TLSv1.3` |
| `VersionTLS11` | `-ALL +TLSv1.1 +TLSv1.2 +TLSv1.3` |
| `VersionTLS12` | `-ALL +TLSv1.2 +TLSv1.3` |
| `VersionTLS13` | `-ALL +TLSv1.3` |

`minTLSVersion` of `VersionTLS10` or `VersionTLS11` emits warning event `DeprecatedVirtualMediaServerTLSVersion` on the Provisioning object while virtual-media TLS is enabled. `virtualMediaServerTLS` cannot be set when `disableVirtualMediaTLS` is true. The API rejects that update with `virtualMediaServerTLS cannot be set when disableVirtualMediaTLS is true`.

The virtual-media HTTPS listener is port 6183 on `metal3-httpd`. The Ironic API listener is port 6385. The baremetal-operator container arguments (`--tls-min-version`, `--tls-cipher-suites`, `--tls-curve-preferences`) are not part of this override.

### 1.2 References

- [METAL-1947](https://redhat.atlassian.net/browse/METAL-1947), including the decision to configure the existing virtual-media server in ironic-image and to leave Ironic's BMC client unchanged.
- [METAL-1994](https://redhat.atlassian.net/browse/METAL-1994), the component spike.
- [METAL-1999](https://redhat.atlassian.net/browse/METAL-1999), the implementation story.
- [METAL-1946](https://redhat.atlassian.net/browse/METAL-1946), for the `groups` field only.
- `VirtualMediaServerTLS` in `api/v1alpha1/provisioning_types.go` and `Validate()` in `api/v1alpha1/provisioning_validation.go`.
- `warnDeprecatedVirtualMediaServerTLS` in `controllers/provisioning_controller.go`.
- `createContainerMetal3Httpd` in `provisioning/baremetal_pod.go`.
- IEEE Std 829-2008, IEEE Standard for Software and System Test Documentation.

Those Go identifiers are a prerequisite. [METAL-1999](https://redhat.atlassian.net/browse/METAL-1999) already added them on `main`. This change adds the system tests and this plan, and a checkout of only this change does not contain that implementation.

### 1.3 Test overview

Testing has three layers.

Component tests in this repository check admission and the environment variables written onto `metal3-httpd`. They run with `make unit`.

System tests in `tests-extension/openshift/test/e2e/virtual_media_server_tls.go` run against a live bare-metal cluster. They check the running listener, isolation from the Ironic API and the baremetal operator, and a worker reprovision. The suite skips when `provisioning.spec.virtualMediaServerTLS` is absent from the CRD.

The cases in section 2.2 that are marked Manual cover behavior the automated system tests do not assert: the deprecated-version warning, the full minimum-version matrix, an omitted cipher family, and the FIPS limits on TLS 1.0 and 1.1.

## 2. Details

### 2.1 Test items

| Identifier | Item |
|---|---|
| TI-API | `Provisioning.spec.virtualMediaServerTLS` admission and CRD validation |
| TI-CBO | Cluster Baremetal Operator reconcile of the metal3 deployment |
| TI-HTTPD | `metal3` deployment, container `metal3-httpd`, label `baremetal.openshift.io/cluster-baremetal-operator=metal3-state` |
| TI-LISTEN | Virtual-media HTTPS listener on port 6183 |
| TI-ISOLATION | Ironic API listener on port 6385 and `metal3-baremetal-operator` arguments |
| TI-BOOT | One MachineSet worker whose BMC address contains `virtualmedia` |

### 2.2 Test traceability

| Case | Requirement | Item | Execution | Automated by |
|---|---|---|---|---|
| TC-VMEDIA-TLS-001 | Unset object keeps the virtual-media listener on the cluster profile | TI-HTTPD | Automated | Level0 e2e |
| TC-VMEDIA-TLS-002 | Each `minTLSVersion` maps to the protocol string in section 1.1 | TI-API, TI-HTTPD | Automated for `VersionTLS12`; manual for the other three | Disruptive e2e, in part |
| TC-VMEDIA-TLS-003 | Cipher names split into the TLS 1.2 and TLS 1.3 variables | TI-HTTPD, TI-LISTEN | Automated | Disruptive e2e |
| TC-VMEDIA-TLS-004 | `groups` sets `IRONIC_VMEDIA_CURVES` only | TI-HTTPD | Automated | Disruptive e2e |
| TC-VMEDIA-TLS-005 | Invalid version, cipher, group, and empty lists are rejected | TI-API | Automated for a bad version, a cipher containing a space, and an unknown group | Disruptive e2e, in part |
| TC-VMEDIA-TLS-006 | The object cannot be combined with `disableVirtualMediaTLS: true` | TI-API | Automated for both directions of the resulting object | Disruptive e2e |
| TC-VMEDIA-TLS-007 | TLS 1.0 and 1.1 emit `DeprecatedVirtualMediaServerTLSVersion` | TI-CBO | Manual | — |
| TC-VMEDIA-TLS-008 | Ironic API settings and baremetal-operator arguments stay unchanged | TI-ISOLATION | Automated | Disruptive e2e |
| TC-VMEDIA-TLS-009 | Port 6183 completes a handshake at the configured floor | TI-LISTEN | Automated for TLS 1.2 | Disruptive e2e |
| TC-VMEDIA-TLS-010 | A virtual-media worker deprovisions and provisions while the floor is TLS 1.2, and the image URL uses port 6183 | TI-BOOT, TI-LISTEN | Automated | Reprovision e2e |
| TC-VMEDIA-TLS-011 | Removing the object returns the listener to the previous profile | TI-HTTPD | Automated | Disruptive e2e cleanup |
| TC-VMEDIA-TLS-012 | A cipher list that names only one protocol family leaves the other family enabled | TI-LISTEN | Manual | — |
| TC-VMEDIA-TLS-013 | iPXE TLS variables are not changed by this object | TI-HTTPD | Manual | — |
| TC-VMEDIA-TLS-014 | On FIPS, TLS 1.0 and 1.1 are not treated as a successful handshake | TI-LISTEN | Manual | — |

The automated cases are:

- `Author:jhajyahy-High-METAL-1947-Virtual media TLS listener matches the Provisioning spec [Level0]`
- `Author:jhajyahy-High-METAL-1947-Virtual media TLS override does not change Ironic API TLS [Serial][Disruptive]`
- `Author:jhajyahy-Longduration-NonPreRelease-High-METAL-1947-Worker reprovision uses the virtual-media TLS listener [Serial][Disruptive]`

### 2.3 Features to be tested

- Cluster-profile inheritance when `virtualMediaServerTLS` is absent.
- Minimum version `VersionTLS10` through `VersionTLS13`, including versions above the floor remaining enabled.
- Cipher override, including the `TLS_` split, and group override.
- Rejection of a version outside the enum, a cipher that contains whitespace or a separator, an unknown group, and an empty `ciphers` or `groups` list.
- Mutual exclusion with `disableVirtualMediaTLS`.
- The deprecation warning for TLS 1.0 and TLS 1.1, and the absence of that warning for TLS 1.2 and TLS 1.3.
- Isolation of `IRONIC_SSL_*`, `IRONIC_TLS_CURVES`, and the baremetal-operator TLS arguments.
- A live handshake to port 6183. Pass or fail is decided from the handshake text. `openssl s_client` may exit non-zero after a successful handshake because the listener certificate is signed by a private CA. A result that contains `handshake failure`, `no protocols available`, `alert protocol version`, or `Cipher is (NONE)` is a failure. A result that contains the negotiated cipher is a pass.
- Worker deprovision and provision on a bare-metal IPI cluster, with `:6183` present in `metal3-ironic` or `metal3-httpd` logs from that cycle.
- Restoration of the previous value, including removal of the field.

### 2.4 Features not to be tested

These are outside [METAL-1947](https://redhat.atlassian.net/browse/METAL-1947):

- Ironic's outbound TLS when it calls a BMC Redfish API, including virtual-media insert and eject calls.
- A separate maximum TLS version.
- Per-host or per-BMC settings.
- Turning virtual media onto plain HTTP. That remains `disableVirtualMediaTLS`.
- The iPXE vhost TLS policy, except the check that this object does not modify it.
- Baremetal-operator curve preferences toward the Kubernetes API server. [METAL-1946](https://redhat.atlassian.net/browse/METAL-1946) covers that client. This plan only checks that `virtualMediaServerTLS.groups` does not change those arguments.
- A BMC firmware upgrade, and a BMC that can speak only TLS 1.0 or 1.1. The automated boot uses a TLS 1.2 floor so a normal CI BMC can complete it. A TLS 1.1-only BMC is a hardware-specific follow-up, not a pass criterion for this plan.
- A BareMetalHost status condition that lists attempted TLS versions. The epic does not require one.

### 2.5 Approach

Component tests run in CI with `make unit` on a payload that contains the field. They do not need hardware.

System tests run from `tests-extension` against a bare-metal IPI cluster. The Level0 case is read-only and may run in parallel. The override case and the reprovision case are `[Serial][Disruptive]`: each one patches the singleton Provisioning object, and the reprovision case also scales a MachineSet. They must not run at the same time.

The reprovision case assumes real virtual-media BMCs, the hardware used by bare-metal IPI CI. It selects a worker whose `spec.bmc.address` contains `virtualmedia` (`redfish-virtualmedia`, `idrac-virtualmedia`, or `ilo5-virtualmedia`) and whose Machine is owned by a MachineSet. A PXE worker is a skip. dev-scripts is not the target environment: its BMC emulator does not fetch the image from port 6183 the way cluster hardware does.

Before a disruptive case changes the object, record `metal3-httpd` environment and the MachineSet replica count. Restore the replica count first, wait until the BareMetalHost is `provisioned` and the node is Ready, then restore `virtualMediaServerTLS` and wait until `IRONIC_VMEDIA_SSL_PROTOCOL` matches the recorded value.

On a FIPS cluster, execute TC-VMEDIA-TLS-014 instead of treating a TLS 1.0 or 1.1 handshake failure as a product defect. TLS 1.2 with `ECDHE-RSA-AES128-GCM-SHA256` and group `secp384r1` is the FIPS-safe functional override. `X25519MLKEM768` is not a FIPS pass criterion.

Read listener environment with `oc exec` of `/usr/bin/env` in `metal3-httpd`. Do not use a shell as the container command. A CI debug pod can fail `getpwuid` before the command runs; that failure is in the debug pod, not in the listener.

### 2.6 Item pass/fail criteria

The feature passes when every case in section 2.2 has passed or has an agreed skip.

A case passes when the expected API result, environment, handshake, event, or host state in section 2.9 is observed and the cluster is returned to its starting Provisioning spec and MachineSet size.

A case fails when admission accepts an invalid object, the virtual-media variables do not match the object, an `IRONIC_*` variable or a baremetal-operator argument changes, port 6183 negotiates a version below the configured floor, the warning is missing for TLS 1.0 or 1.1, or the reprovisioned host does not return to `provisioned` with `:6183` in the provision logs.

A skip is allowed when the CRD field is absent, `disableVirtualMediaTLS` is already true, the cluster is not bare metal, or no virtual-media worker is owned by a MachineSet. A skip of TC-VMEDIA-TLS-010 on a PXE-only pool does not fail the plan. The case remains required on a virtual-media IPI pool.

### 2.7 Suspension criteria and resumption requirements

Stop the disruptive and reprovision cases when patching the Provisioning object leaves `baremetal` `ClusterOperator` degraded, when `metal3-httpd` does not become Ready within 6 minutes of a valid patch, or when the MachineSet cannot be scaled back to its original replica count.

Resume after the Provisioning object is restored, `metal3-httpd` is Ready, the worker BareMetalHost is `provisioned`, and the node and cluster operators are healthy. Do not start the next disruptive case while a previous override is still set.

### 2.8 Test deliverables

- This plan.
- The component tests in `api/v1alpha1/provisioning_validation_test.go` and `provisioning/baremetal_pod_test.go`.
- The system tests in `tests-extension/openshift/test/e2e/virtual_media_server_tls.go`.
- The system-test log and, for a failure, the Provisioning object, the `metal3-httpd` environment, and the relevant `metal3-ironic` log excerpt.
- A result against [METAL-2009](https://redhat.atlassian.net/browse/METAL-2009) once the manual cases have been executed.

### 2.9 Test case outline

Each case lists the IEEE 829 inputs and expected results. Special procedures are the record-and-restore rule in section 2.5. TC-VMEDIA-TLS-010 depends on TC-VMEDIA-TLS-009 having established that port 6183 honors the TLS 1.2 floor. No other case depends on another case's leftover state.

#### TC-VMEDIA-TLS-001 Cluster profile inheritance

Input: `virtualMediaServerTLS` absent or null, and `disableVirtualMediaTLS` false.

Expected result: `IRONIC_VMEDIA_SSL_PROTOCOL`, `IRONIC_VMEDIA_TLS_12_CIPHERS`, `IRONIC_VMEDIA_TLS_13_CIPHERS`, and `IRONIC_VMEDIA_CURVES` equal the matching `IRONIC_SSL_PROTOCOL`, `IRONIC_TLS_12_CIPHERS`, `IRONIC_TLS_13_CIPHERS`, and `IRONIC_TLS_CURVES` values.

#### TC-VMEDIA-TLS-002 Minimum version

Input: one patch per row of the protocol table in section 1.1, with no `ciphers` or `groups`.

Expected result: after `metal3-httpd` rolls out, `IRONIC_VMEDIA_SSL_PROTOCOL` equals that row. For `VersionTLS12`, a TLS 1.2 handshake to port 6183 succeeds and a TLS 1.3 handshake still succeeds. For `VersionTLS13`, a TLS 1.2 handshake to port 6183 fails and a TLS 1.3 handshake succeeds.

#### TC-VMEDIA-TLS-003 Cipher split

Input: `minTLSVersion: VersionTLS12` and `ciphers: [ECDHE-RSA-AES128-GCM-SHA256, TLS_AES_128_GCM_SHA256]`.

Expected result: `IRONIC_VMEDIA_TLS_12_CIPHERS` is `ECDHE-RSA-AES128-GCM-SHA256` and `IRONIC_VMEDIA_TLS_13_CIPHERS` is `TLS_AES_128_GCM_SHA256`. A TLS 1.2 handshake reports `ECDHE-RSA-AES128-GCM-SHA256`.

#### TC-VMEDIA-TLS-004 Groups

Input: `minTLSVersion: VersionTLS12` and `groups: [secp384r1]`.

Expected result: `IRONIC_VMEDIA_CURVES` is `secp384r1`. `IRONIC_TLS_CURVES` and the baremetal-operator arguments stay at their recorded values.

#### TC-VMEDIA-TLS-005 Rejection

Input, each as its own dry-run patch:

- `minTLSVersion: TLSv1.2`
- `minTLSVersion` omitted
- `ciphers: []`
- `ciphers: ["ECDHE RSA"]`
- `groups: []`
- `groups: ["not-a-group"]`

Expected result: the API rejects every patch. The stored object does not change. The version error names `VersionTLS10`, `VersionTLS11`, `VersionTLS12`, and `VersionTLS13`. The cipher error says the entry must be an OpenSSL cipher name without whitespace or separators. The group error says the entry is not a supported TLS group.

#### TC-VMEDIA-TLS-006 Mutual exclusion

Input, as dry-run patches:

- `disableVirtualMediaTLS: true` together with `virtualMediaServerTLS`, so admission sees both fields on the resulting object.
- `disableVirtualMediaTLS: true` after `virtualMediaServerTLS` is already stored.
- When `disableVirtualMediaTLS` is already true, `virtualMediaServerTLS` by itself. The rest of the disruptive test then skips, because the HTTPS listener is off.

Expected result: each update fails with `virtualMediaServerTLS cannot be set when disableVirtualMediaTLS is true`. The automated test does not store `disableVirtualMediaTLS: true` on a cluster where it is currently false, because that would remove the HTTPS listener.

#### TC-VMEDIA-TLS-007 Deprecation warning

Input: `minTLSVersion` set to `VersionTLS10`, then to `VersionTLS11`, then to `VersionTLS12`, with `disableVirtualMediaTLS` false.

Expected result: the first two updates emit a Warning event on `provisioning-configuration` with reason `DeprecatedVirtualMediaServerTLSVersion` and a message that names the version. `VersionTLS12` and `VersionTLS13` do not emit that reason.

#### TC-VMEDIA-TLS-008 Isolation

Input: the override from TC-VMEDIA-TLS-003 plus `groups: [secp384r1]`.

Expected result: `IRONIC_SSL_PROTOCOL`, `IRONIC_TLS_12_CIPHERS`, `IRONIC_TLS_13_CIPHERS`, and `IRONIC_TLS_CURVES` equal the values recorded before the patch. The baremetal-operator arguments equal the recorded arguments and do not contain `VMEDIA`. When the recorded API protocol is `-ALL +TLSv1.3`, port 6385 first completes a TLS 1.3 handshake. The following TLS 1.2 handshake is a rejection only when its output contains a failure signature from section 2.3. A missing cipher, a refused connection, or a failed `oc exec` is not a rejection. Port 6183 still accepts TLS 1.2.

#### TC-VMEDIA-TLS-009 Listener handshake

Input: the TLS 1.2 override, then `openssl s_client -connect 127.0.0.1:6183 -tls1_2` executed in the running `metal3` pod.

Expected result: the handshake text contains `ECDHE-RSA-AES128-GCM-SHA256` and does not contain a failure signature from section 2.3. The process exit code is not the pass criterion.

#### TC-VMEDIA-TLS-010 Worker reprovision

Input: a bare-metal IPI cluster, `minTLSVersion: VersionTLS12`, and one virtual-media worker Machine owned by a MachineSet with at least one replica.

Steps:

1. Wait until `IRONIC_VMEDIA_SSL_PROTOCOL` is `-ALL +TLSv1.2 +TLSv1.3`.
2. Annotate the Machine with `machine.openshift.io/cluster-api-delete-machine=yes`.
3. Scale that MachineSet down by one and wait until the BareMetalHost provisioning state is `available`.
4. Scale the MachineSet back to its original replica count and wait until the state is `provisioned`.
5. Collect `metal3-ironic` and `metal3-httpd` logs from one minute before the scale-up.

Expected result: the host reaches `provisioned`, the new node becomes Ready, cluster operators are healthy, and the combined logs contain `:6183`. The MachineSet replica count and the original `virtualMediaServerTLS` value are restored.

#### TC-VMEDIA-TLS-011 Restore

Input: a cluster left with a virtual-media override. Remove `/spec/virtualMediaServerTLS`.

Expected result: the listener variables return to the values recorded before the override, as in TC-VMEDIA-TLS-001.

#### TC-VMEDIA-TLS-012 Omitted cipher family

Input: `minTLSVersion: VersionTLS12` and `ciphers: [ECDHE-RSA-AES128-GCM-SHA256]`, with no `TLS_` name.

Expected result: the TLS 1.2 listener uses `ECDHE-RSA-AES128-GCM-SHA256`. A TLS 1.3 handshake to port 6183 still succeeds, because the omitted TLS 1.3 family stays enabled.

#### TC-VMEDIA-TLS-013 iPXE isolation

Input: any valid `virtualMediaServerTLS` patch.

Expected result: `IPXE_SSL_PROTOCOL`, `IPXE_TLS_12_CIPHERS`, and `IPXE_TLS_13_CIPHERS` on `metal3-httpd` are unchanged. This object does not set an iPXE variable.

#### TC-VMEDIA-TLS-014 FIPS

Input: a FIPS cluster and `minTLSVersion: VersionTLS10`.

Expected result: the warning from TC-VMEDIA-TLS-007 is present and `IRONIC_VMEDIA_SSL_PROTOCOL` is the TLS 1.0 row from section 1.1. A failed TLS 1.0 or 1.1 handshake is recorded and is not a failure of this case. The functional pass on this cluster remains the TLS 1.2 handshake from TC-VMEDIA-TLS-009.

## 3. Test management

### 3.1 Planned activities

1. Confirm the payload CRD exposes `provisioning.spec.virtualMediaServerTLS`.
2. Run `make unit` for TI-API and TI-CBO.
3. Run the Level0 system test.
4. Run the disruptive system test, then confirm the object was restored.
5. Run the reprovision system test on a bare-metal IPI pool that has a virtual-media worker.
6. Execute the manual cases TC-VMEDIA-TLS-002 (except `VersionTLS12`), TC-VMEDIA-TLS-005 (empty lists and omitted version), TC-VMEDIA-TLS-007, TC-VMEDIA-TLS-012, TC-VMEDIA-TLS-013, and TC-VMEDIA-TLS-014.
7. Record results on [METAL-2009](https://redhat.atlassian.net/browse/METAL-2009).

### 3.2 Environment

- An OpenShift 5.1 cluster installed with a payload that contains the merged `virtualMediaServerTLS` API. The test level is the installed cluster, not a unit-test fake.
- Platform `baremetal`. TC-VMEDIA-TLS-010 needs an IPI MachineSet and a worker BMC address containing `virtualmedia`.
- `disableVirtualMediaTLS` left false for every case except the mutual-exclusion case.
- Admin `oc` credentials. Handshake and environment checks run with `oc exec` inside the `metal3` pod.
- The reprovision case needs a worker pool that can lose one replica and return. Compact clusters without a worker MachineSet skip that case.
- Do not run the disruptive cases on a production cluster. A custom operator image requires the usual cluster-version override and a restart of the operator after the image ConfigMap changes.

### 3.3 Responsibilities

| Role | Responsibility |
|---|---|
| QE, Jad Haj Yahya | Maintain this plan, run the system and manual cases, and report the result on METAL-2009 |
| Development, Himanshu Roy | Keep the component tests aligned with `VirtualMediaServerTLS.Validate` and the `metal3-httpd` environment |
| Epic owner | Accept failures that show the listener, the admission rules, or the worker boot disagree with section 1.1 |

### 3.4 Interfaces

QE depends on the Cluster Baremetal Operator build that ships in the 5.1 payload and on ironic-image rendering `IRONIC_VMEDIA_SSL_PROTOCOL`, `IRONIC_VMEDIA_TLS_12_CIPHERS`, `IRONIC_VMEDIA_TLS_13_CIPHERS`, and `IRONIC_VMEDIA_CURVES` into the virtual-media Apache vhost. A failure confined to Apache ignoring a variable that CBO set correctly is an ironic-image defect, and the CBO case still fails until the listener negotiates the configured policy.

### 3.5 Resources

One bare-metal IPI cluster for the serial cases, plus the existing unit-test CI job. The reprovision case holds one worker for the deprovision and provision interval, which can take up to the 90 minute BareMetalHost wait plus the node and operator health checks.

### 3.6 Training

The tester needs to read a Provisioning object, a container environment, and an `openssl s_client` result, and to scale a MachineSet back after a failed reprovision. No separate product training is required.

### 3.7 Schedule

System and manual execution starts on the first openshift-5.1 payload whose CRD contains `virtualMediaServerTLS`. Component tests are already part of `make unit` on that payload. [METAL-2009](https://redhat.atlassian.net/browse/METAL-2009) closes after section 3.1 is complete, including the manual cases.

### 3.8 Risks and contingencies

| Risk | Contingency |
|---|---|
| The payload predates the field | The system suite skips. Do not record a pass. |
| A FIPS node rejects TLS 1.0 inside `metal3-httpd` even though the variable is set | Apply TC-VMEDIA-TLS-014. Do not lower the cluster TLS profile to force the handshake. |
| The reprovision runs on a pool that boots with PXE | Skip TC-VMEDIA-TLS-010. Run it on a `virtualmedia` pool before closing METAL-2009. |
| `openssl s_client` exits non-zero because the private CA is untrusted | Judge the cipher line, as specified in section 2.3. |
| A failed scale-down leaves a Machine annotated for deletion | Remove `machine.openshift.io/cluster-api-delete-machine` if the scale did not happen. If the scale happened, restore the original replica count and wait for `provisioned` before removing the TLS override. |
| The warning event is deduplicated by Kubernetes | Delete the previous warning, or change from `VersionTLS10` to `VersionTLS11`, before checking the second emission. |

## 4. General

### 4.1 Quality assurance

A case result is accepted when the command output cited in section 2.9 is retained with the build identifier. Dry-run checks use `--dry-run=server` so they exercise admission rather than client-side schema checks alone. A pass recorded only from a unit test does not close a system case in section 2.2.

### 4.2 Metrics

- Cases executed, passed, failed, and skipped, out of the 14 cases in section 2.2.
- For TC-VMEDIA-TLS-010, time from scale-up to BareMetalHost `provisioned`.

### 4.3 Test coverage

Section 2.2 maps each METAL-1947 behavior in section 1.1 to a case. Automated coverage is complete for inheritance, one minimum version, cipher split, one group, the common admission rejects, mutual exclusion of the resulting object in both directions, API and baremetal-operator isolation, the TLS 1.2 handshake, worker reprovision, and restore. The remaining minimum versions, the warning, the omitted cipher family, iPXE isolation, and the FIPS interpretation stay manual until they are added to the system suite.

### 4.4 Glossary

| Term | Meaning |
|---|---|
| Virtual-media listener | The `metal3-httpd` HTTPS vhost on port 6183 that serves the image a BMC mounts |
| Floor | The minimum TLS version. Higher versions through TLS 1.3 stay enabled |
| Cluster profile | The TLS profile CBO applies when `virtualMediaServerTLS` is unset |
| IPI | Installer-provisioned infrastructure, with MachineSets and BareMetalHosts |

### 4.5 Document history

| Version | Date | Description |
|---|---|---|
| 1.0 | 2026-11-16 | Initial level test plan for METAL-1947, aligned with the merged virtual-media server API and the system tests on branch `virtual-media-server-tls`. |
| 1.1 | 2026-11-17 | Record METAL-1999 on `main` as the implementation prerequisite. State that mutual exclusion is automated for both directions of the resulting object, and that a TLS 1.2 rejection requires an explicit handshake failure. |

## 5. Approvals

| Role | Name | Date | Result |
|---|---|---|---|
| QE | Jad Haj Yahya | | |
| Development | Himanshu Roy | | |
