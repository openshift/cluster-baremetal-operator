# [5.1] Fast Out-of-Band Inspection

## Test Plan Identifier
[`OCPSTRAT-3566-test-plan`](https://redhat.atlassian.net/browse/OCPSTRAT-3566)

---

## 1. Introduction

### Purpose
This test plan validates the implementation of fast out-of-band inspection for bare metal hosts in the Metal Platform. The feature introduces a new inspection mode in openshift-5.1 that uses Redfish API calls instead of booting Ironic Python Agent (ramdisk) on the target machine, providing a faster and less invasive method of hardware inventory collection.

### Scope

**In Scope:**
- Fast inspection via Redfish API for BareMetalHost resources
- Hardware inventory collection without booting any payload on the target machine
- Performance validation (desired goal of within 1 minute, but may have exceptions)
- Inventory field parity with agent-based inspection (where Redfish data is available)
- Re-inspection annotation behavior with fast inspection configuration
- Interoperability with existing BareMetalHost workflows
- Use as an alternative to agent-based inspection

**Out of Scope:**
- Non-Redfish implementation methods
- OEM-specific Redfish fields or vendor-proprietary extensions
- Complete feature parity with agent-based inspection
- Modifications to the IPI installation flow
- Fields requiring vendor-specific OEM resources
- GPU discovery is expected, but no hardware with enterprise-grade GPUs are available to test

---

## 2. Background / References

- **OCPSTRAT Issue:** [OCPSTRAT-3566](https://redhat.atlassian.net/browse/OCPSTRAT-3566) - Fast out-of-band inspection in Metal platform
- **Component:** [openshift/baremetal-operator](https://github.com/openshift/baremetal-operator) (BareMetalHost API)
- **Related Documentation:** [OpenShift Bare Metal IPI Documentation](https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal_ipi/ipi-install-overview.html)
- **Upstream Projects:** [Metal3 Baremetal Operator](https://github.com/metal3-io/baremetal-operator)

**Context:**
The Metal Platform currently supports hardware inspection by booting Ironic Python Agent (IPA) on target machines. This approach is comprehensive but time-consuming and requires booting a payload onto the machine. Fast inspection addresses use cases where hardware verification is needed before booting anything (e.g., auto-discovery, enrollment verification) by using out-of-band Redfish API calls.

---

## 3. Test Items

- **BareMetalHost API** - New inspection mode configuration
- **Baremetal Operator** - Fast inspection controller logic
- **Redfish Client** - Hardware inventory collection via Redfish
- **Hardware Data Schema** - Inventory field mapping and population
- **Re-inspection Annotation** - Configuration persistence

---

## 4. Features to Be Tested

### Functional Requirements (from Acceptance Criteria)
1. **FR-01:** Fast inspection completes within 1 minute with nominal BMC response times (< 0.1 sec/response)
2. **FR-02:** Fast inspection does not boot any payload on the target machine
3. **FR-03:** Inventory fields available from agent-based inspection are also available from fast inspection when Redfish exposes them in a vendor-neutral way
4. **FR-04:** Re-inspection annotation respects the inspection mode configuration (fast vs agent)

### User Goals
5. **UG-01:** User can request that a new BareMetalHost uses fast inspection instead of agent-based inspection
6. **UG-02:** User can verify machine hardware before booting any payload (auto-discovery use case)
7. **UG-03:** User can make provisioning decisions based on fast inspection results

### Inventory Coverage
8. **INV-01:** CPU information (architecture, model, count, clock speed)
9. **INV-02:** Memory capacity
10. **INV-03:** Network interfaces (MAC addresses, names, speed)
11. **INV-04:** Storage devices (model, size, vendor, serial number, type)
12. **INV-05:** System vendor information (manufacturer, product name, serial number)
13. **INV-06:** Firmware information (validate with HostFirmwareComponents as HardwareData does not have enough firmware information)
14. **INV-07:** Server Model

---

## 5. Features Not to Be Tested

The following are explicitly excluded from the scope of this test plan:

1. **Non-Redfish Implementations:** IPMI, custom vendor protocols, or other out-of-band management protocols
2. **OEM-Specific Fields:** Vendor-proprietary Redfish extensions or OEM resource fields
3. **Complete Feature Parity:** Fields that are only available through in-band inspection (e.g., kernel-level hardware detection)
4. **IPI Flow Modifications:** Changes to the installer-provisioned infrastructure installation process
5. **Agent-Based Inspection:** Existing ramdisk-based inspection (regression testing only)

---

## 6. Approach

### Testing Strategy

**Manual Testing:**
- Initial functional validation
- Hardware compatibility verification across vendors
- Performance measurement under various BMC conditions
- User workflow validation

**Automated Testing:**
- CI/CD integration for regression detection
- Unit tests for Redfish client and inventory mapping
- End-to-end tests with virtual BMC (Sushy-tools) or real BMC of bare metal hosts

**Environment Tiers:**
- **Development:** Local clusters with virtual BMC (Sushy-tools)
- **CI:** Automated test suites with mock/virtual infrastructure
- **Staging:** Real bare metal hardware (Dell, HPE)

### Test Phases

1. **Functional Validation** - Core feature functionality and acceptance criteria
2. **Platform Variations** - Vendor compatibility, performance, and configuration testing
3. **Integration & Regression** - Workflow integration and regression validation

---

## 7. Item Pass/Fail Criteria

| Test Item | Pass Criteria | Fail Criteria |
|-----------|---------------|---------------|
| **Fast Inspection Performance** | Completes within 1 minute with nominal BMC response (< 0.1 sec/response); within 3 minutes with slow BMC (0.5 sec/response) | Exceeds time limits or times out |
| **No Payload Boot** | No boot/reboot activity detected via BMC monitoring, console logs, or system events for powered-off and powered-on hosts | Machine boots any payload or shows boot activity |
| **Inventory Field Parity** | Vendor-neutral Redfish fields populate matching agent-based fields | Missing fields available in both methods |
| **Re-inspection Annotation** | Re-inspection uses configured mode | Ignores configuration or uses wrong mode |
| **Hardware Data Schema** | Populates HardwareData with correct types and values | Schema validation errors or incorrect types |
| **BMC Compatibility** | Works with DMTF-compliant Redfish | Fails on standard-compliant BMC |
| **Error Handling** | Gracefully handles failures; recovers when BMC restored | Crashes, hangs, or corrupts data |

---

## 8. Suspension Criteria and Resumption Requirements

### Suspension Criteria
1. **Critical Infrastructure Failure:** Test environment unavailable
2. **Blocking Defects:** Fast inspection causes boots, corrupts state, or BMO crashes
3. **API Breaking Changes:** BareMetalHost API changes require plan revision

### Resumption Requirements
1. Infrastructure restored and validated
2. Blocking defects fixed and verified
3. API changes documented and plan updated

---

## 9. Test Deliverables

1. **Test Plan Document:** This IEEE 829-style test plan
2. **Test Case Coverage Mapping:** Test case IDs with requirement traceability (see Section 17)
3. **Test Results:** Pass/fail status with evidence and defect reports
4. **Performance Metrics:** Inspection times across hardware vendors
5. **Compatibility Matrix:** Hardware vendor/model results
6. **CI Automation:** Test scripts and CI configurations

---

## 10. Testing Tasks

1. **Environment Setup** - Provision clusters with Metal3 and BareMetalHost CRDs, configure virtual/real BMC (Dell, HPE)
2. **Test Case Development** - Author detailed specifications for all coverage categories
3. **Test Execution** - Execute functional, deployment, interoperability, performance, and regression suites
4. **Results Analysis** - Analyze results, categorize defects, compare vendor performance
5. **Defect Management** - File and track Jira issues
6. **CI/CD Integration** - Integrate automated tests into pipeline

---

## 11. Environmental Needs

### Infrastructure Requirements

**Cluster Configurations:**
- OpenShift clusters with Metal3 operator and BareMetalHost CRD
- Network connectivity to BMC endpoints

**Hardware Requirements:**
- Bare metal systems (Dell PowerEdge 14th gen+, HPE ProLiant Gen10+) with Redfish 1.6+
- Virtual BMC (Sushy-tools) for mock testing

**Network:**
- Isolated VLAN for BMC traffic
- Routing to BMC management network

---

## 12. Responsibilities

| Role | Testing Responsibilities |
|------|--------------------------|
| **QE Team** | Test planning, execution, analysis, defect reporting |
| **Development Team** | Unit tests, feature verification, defect fixes |
| **Release Engineering** | CI/CD integration, release criteria validation |
| **Product Management** | Acceptance criteria, scope approval |

---

## 13. Staffing and Training Needs

N/A

---

## 14. Schedule

| Milestone | Target |
|-----------|--------|
| **Test Plan Approval** | TBD |
| **Test Environment Ready** | TBD |
| **Functional Testing Complete** | TBD |
| **CI Integration Complete** | TBD |

---

## 15. Risks and Contingencies

| Risk | Mitigation Strategy |
|------|---------------------|
| **Hardware Availability** | Prioritize testing with real bare metal hardware |
| **Redfish Implementation Variance** | Test across vendors early; document vendor-specific issues |
| **Incomplete Redfish Data** | Document per-vendor field availability; accept partial parity |
| **Performance Variability** | Use configurable timeout; test with simulated latency |
| **API Changes During Development** | Track changes via PRs; update plan as needed |

---

## 16. Approvals

This test plan requires approval from the following stakeholders:

- **QE Lead:** Steeve Goveas - Test plan completeness
- **QE Owner:** Salwa Naoum - Test execution and quality validation
- **Feature Owner:** Dmitry Tantsur - Acceptance criteria alignment
- **Development Lead:** Dmitry Tantsur - Technical feasibility and implementation approach
- **Product Management:** Michal Zasepa - Business requirements

**Approval Date:** TBD

---

## 17. Test Coverage Mapping

This section provides test coverage mapping with test case IDs, titles, and requirement traceability.

---

#### 17.1 Functional Validation

**Source:** Functional Requirements (Acceptance Criteria)

| Test Case ID | Title | Requirement Traceability | Coverage |
|--------------|-------|--------------------------|----------|
| TC-FUNC-01 | Fast inspection completes within 1 minute | FR-01 | Complete |
| TC-FUNC-02 | Fast inspection does not boot any payload | FR-02 | Complete |
| TC-FUNC-03 | CPU information collected via Redfish | FR-03, INV-01 | Complete |
| TC-FUNC-04 | Memory capacity collected via Redfish | FR-03, INV-02 | Complete |
| TC-FUNC-05 | Network interfaces collected via Redfish | FR-03, INV-03 | Complete |
| TC-FUNC-06 | Storage devices collected via Redfish | FR-03, INV-04 | Complete |
| TC-FUNC-07 | System vendor information collected via Redfish | FR-03, INV-05 | Complete |
| TC-FUNC-08 | Firmware information collected via Redfish | FR-03, INV-06 | Complete |
| TC-FUNC-09 | Re-inspection respects fast mode configuration | FR-04 | Complete |
| TC-FUNC-10 | User configures BareMetalHost for fast inspection | UG-01 | Complete |
| TC-FUNC-11 | Fast inspection HardwareData schema validation | FR-03 | Complete |
| TC-FUNC-12 | Fast inspection state transitions in BMH status | UG-01 | Complete |

---

#### 17.2 Testing and Validation

N/A - Covered by Functional Validation test cases above.

---

#### 17.3 Deployment/Topology Variations

**Note:** The feature does not specify deployment topology restrictions. Fast inspection is a BareMetalHost-level feature that should work consistently across OpenShift deployment types. Testing will focus on BMC/hardware variations rather than cluster topology.

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-DEPLOY-01 | Fast inspection on multi-node bare metal cluster | Complete |
| TC-DEPLOY-02 | Fast inspection on compact 3-node cluster | Complete |
| TC-DEPLOY-03 | Fast inspection on Single Node OpenShift (SNO) | Complete |

---

#### 17.4 Interoperability

| Test Case ID | Title | Requirement Traceability | Coverage |
|--------------|-------|--------------------------|----------|
| TC-INTEROP-01 | Fast inspection as alternative to agent-based inspection | Interop Use Case 1 | Complete |
| TC-INTEROP-02 | Fast inspection with existing provisioning workflows | General interop | Complete |

---

#### 17.5 Upgrade/Rollback

N/A - Feature is additive; agent-based inspection remains default.

---

#### 17.6 Success Criteria

| Success Metric | Test Case ID | Pass Criteria | Coverage |
|----------------|--------------|---------------|----------|
| Fast inspection completes successfully | TC-FUNC-01 | Completes within time limits | Complete |
| No machine boots during inspection | TC-FUNC-02 | No boot activity detected | Complete |
| Inventory data populated | TC-FUNC-03 to TC-FUNC-08 | HardwareData populated with required fields | Complete |

---

#### 17.7 Non-Functional Requirements

| NFR Category | Test Case ID | Title | Coverage |
|--------------|--------------|-------|----------|
| **Performance** | TC-NFR-PERF-01 | Inspection completes within 1 minute | Complete |
| **Performance** | TC-NFR-PERF-02 | Performance with slow BMC (0.5 sec/response): completes within 3 minutes | Complete |
| **Performance** | TC-NFR-PERF-03 | Performance comparison vs agent inspection | Complete |
| **Reliability** | TC-NFR-REL-01 | Graceful BMC timeout handling | Complete |
| **Reliability** | TC-NFR-REL-02 | Retry logic for transient failures | Complete |
| **Reliability** | TC-NFR-REL-03 | Recovery from incomplete Redfish data | Complete |
| **Compatibility** | TC-NFR-COMPAT-01 | Dell hardware compatibility (Redfish 1.6+) | Complete |
| **Compatibility** | TC-NFR-COMPAT-02 | HPE hardware compatibility (Redfish 1.6+) | Complete |
| **Compatibility** | TC-NFR-COMPAT-03 | Redfish version compatibility (1.6, 1.8, 1.10) | Complete |
| **Security** | TC-NFR-SEC-01 | BMC credentials secure storage | Complete |
| **Security** | TC-NFR-SEC-02 | TLS/HTTPS certificate verification: valid cert succeeds, invalid cert rejected | Complete |
| **Usability** | TC-NFR-USE-01 | Clear error messages | Complete |
| **Usability** | TC-NFR-USE-02 | Status reporting during inspection | Complete |

---

#### 17.8 Operational / Day-2

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-OPS-01 | Inspection failure troubleshooting workflow | Partial |
| TC-OPS-02 | BMC connectivity diagnostics | Partial |
| TC-OPS-03 | Inspection metrics exposure | Gap |
| TC-OPS-04 | Logging for inspection failures | Partial |

---

#### 17.9 Negative Tests

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-NEG-01 | OEM-specific fields not required for fast inspection | Complete |
| TC-NEG-02 | Non-Redfish protocols not supported | Complete |
| TC-NEG-03 | IPI flow remains unchanged | Complete |
| TC-NEG-04 | Missing Redfish fields do not block inspection | Complete |

---

#### 17.10 Regression Scenarios

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-REG-01 | Agent-based inspection still works (default behavior) | Complete |
| TC-REG-02 | BareMetalHost provisioning after fast inspection | Complete |
| TC-REG-03 | BareMetalHost provisioning after agent inspection (baseline) | Complete |
| TC-REG-04 | BareMetalHost state machine transitions unchanged | Complete |
| TC-REG-05 | Existing HardwareData schema compatibility | Complete |

---

## 18. Readiness Integration

- Link this test plan from OCPSTRAT-3566
- Store in Git repository: `openshift/baremetal-operator` under `docs/test-plans/` or component quality repository
- Derive QE work items from Testing Tasks (Section 10) and Test Coverage Mapping (Section 17) with traceability to test case IDs

---

**Document Version:** 1.0  
**Last Updated:** 2026-09-15  
**Status:** Draft - Pending Approval
