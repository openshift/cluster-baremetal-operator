# [5.1] OCI Images for BMaaS (including bootc format)

## Test Plan Identifier
`OCPSTRAT-3103-test-plan`

---

## 1. Introduction

### Purpose
This test plan validates the implementation of bootc deploy interface autodetection in OpenShift's bare metal provisioning infrastructure (Ironic) in openshift 5.1. The feature enables Ironic to automatically detect bootc container images and select the appropriate deployment interface, supporting the broader BMaaS (BareMetal as a Service) vision for multi-OS provisioning.

### Scope

**In Scope:**
- Bootc deploy interface autodetection in Ironic
- Automatic interface selection for bootc OCI container images
- Fallback to direct interface for plain OCI artifacts and disk images
- LiveISO provisioning using ramdisk interface (regression validation)
- Integration with Bare Metal Operator (BMO) for bootc deployments
- Interface detection priority order: ramdisk → bootc → direct

**Out of Scope:**
- Multi-OS provisioning beyond bootc (RHEL, Windows - future work)
- Multi-tenancy features (users provisioning BMH without admin privileges)
- OS configuration and post-boot management (delegated to Ansible Automation Platform)
- Custom bootc image build pipelines
- Multi-network provisioning (separate feature)
- Ironic Python Agent (IPA) changes (separate implementation)

---

## 2. Background / References

- **OCPSTRAT Issue:** [OCPSTRAT-3103](https://redhat.atlassian.net/browse/OCPSTRAT-3103) - Delivery image as OCI artefacts (including bootc format)
- **Implementation PR:**
  - [openshift/ironic-image#911](https://github.com/openshift/ironic-image/pull/911) - Enable bootc deploy autodetection
- **Upstream Reference:** [metal3-io/ironic-image#956](https://github.com/metal3-io/ironic-image/pull/956)
- **Component:** Bare Metal Operator (BMO), Metal3, Ironic
- **Related Technologies:** bootc, OCI registries, Ironic deploy interfaces
- **Upstream Ticket:** METAL-1913

**Context:**
OpenShift currently supports bare metal provisioning through Metal3/Ironic integration, primarily for CoreOS deployments. This feature is part of the broader BMaaS vision to enable provisioning of multiple operating systems on bare metal. This test plan focuses specifically on the bootc deploy interface autodetection capability implemented in PR #911, which enables Ironic to automatically select the appropriate deployment interface based on image type.

**Key Implementation Details:**
The PR modifies Ironic's configuration (`ironic.conf.j2`) to:
1. Change `default_deploy_interface` from `direct` to `autodetect`
2. Add `autodetect` to `enabled_deploy_interfaces`
3. Set interface detection order: `autodetect_deploy_interfaces = ramdisk,bootc,direct`

This allows BMO (which doesn't explicitly specify deploy interfaces) to benefit from automatic interface selection:
- **ramdisk**: Selected for LiveISO images (highest priority)
- **bootc**: Selected for bootc OCI container images
- **direct**: Fallback for plain OCI artifacts and standard disk images

---

## 3. Test Items

- **Ironic Configuration** - autodetect interface configuration
- **Ironic Deploy Interfaces** - autodetect, bootc, direct, ramdisk interfaces
- **Bare Metal Operator (BMO)** - Integration with Ironic's autodetect interface
- **BareMetalHost API** - Image specification for different image types
- **Interface Detection Logic** - Automatic selection based on image type

---

## 4. Features to Be Tested

### Functional Requirements (from Implementation PR)
1. **FR-01:** Ironic's default deploy interface is set to autodetect
2. **FR-02:** Interface detection follows ramdisk → bootc → direct priority order
3. **FR-03:** Bootc OCI container images automatically select bootc interface
4. **FR-04:** Plain OCI artifacts and disk images fall back to direct interface
5. **FR-05:** Ramdisk interface has highest priority in autodetect selection order

### Interface Detection and Selection
6. **IF-01:** Autodetect interface is enabled in Ironic configuration
7. **IF-02:** BMO provisioning works without explicitly specifying deploy interface
8. **IF-03:** Interface selection is logged and observable in Ironic

### Integration and Compatibility
9. **INT-01:** Bootc autodetection integrates with existing BMO workflows
10. **INT-02:** Backward compatibility with existing non-bootc deployments
11. **INT-03:** No regressions in CoreOS/RHCOS provisioning

---

## 5. Features Not to Be Tested

The following are explicitly excluded from the scope of this test plan:

1. **Multi-OS Provisioning:** Generic RHEL or Windows provisioning (future feature scope)
2. **IPA Changes:** Ironic Python Agent bootc implementation (separate PR)
3. **Multi-Tenancy:** Non-admin users provisioning BareMetalHost objects
4. **OS Configuration Management:** Post-boot configuration (delegated to other tools)
5. **Image Building:** Creation of bootc container images
6. **Multi-Network Provisioning:** Complex network topologies (separate feature)

---

## 6. Approach

### Testing Strategy

**Manual Testing:**
- Interface detection and selection validation
- Bootc container image provisioning workflows
- Fallback scenarios for different image types
- LiveISO regression testing
- Ironic configuration verification

**Automated Testing:**
- CI integration for interface detection
- Regression tests for existing deployment methods
- Interface selection logging validation

**Environment Tiers:**
1. **Development:** Local test clusters with Metal3 and BMO
2. **CI:** Automated test suites with bootc test images
3. **Staging:** Physical bare metal hardware
4. **Production:** Customer validation

### Test Execution

1. **Functional Validation** - Configuration verification, interface detection, and bootc provisioning
2. **Platform Variations** - Interface selection across different image types and deployment scenarios
3. **Integration & Regression** - BMO workflows and existing deployment method validation

---

## 7. Item Pass/Fail Criteria

| Test Item | Pass Criteria | Fail Criteria |
|-----------|---------------|---------------|
| **Ironic Configuration** | `default_deploy_interface = autodetect` and `autodetect_deploy_interfaces = ramdisk,bootc,direct` configured correctly | Configuration missing or incorrect values |
| **Bootc Interface Detection** | Bootc container images automatically select bootc interface; selection is logged | Bootc images use wrong interface or autodetect fails |
| **Direct Interface Fallback** | Plain OCI artifacts and disk images fall back to direct interface correctly; for OCI indices with missing descriptor metadata (no artifactType or annotations.disktype), autodetect inspects the selected child manifest and correctly classifies as bootc if the child is a bootc artifact, or falls back to direct only if the child manifest is a non-bootc artifact | Direct interface not selected for non-bootc images, or autodetect prematurely falls back to direct without inspecting child manifests when descriptor metadata is missing |
| **Ramdisk Interface Priority** | When deploy_interface is set to autodetect, ramdisk interface is selected first according to detection order (ramdisk → bootc → direct); verified independently of BMO's LiveISO handling | Autodetect fails to prioritize ramdisk interface or selects bootc/direct when ramdisk should be chosen |
| **Bootc Deployment** | Bootc container images deploy successfully when autodetect selects bootc interface; provisioned host boots successfully with the deployed OS | Deployment fails, interface selection incorrect, or provisioned host fails to boot |
| **Interface Logging** | Interface selection decisions are visible in Ironic logs | No logging or insufficient detail for troubleshooting |
| **Backward Compatibility** | Existing CoreOS/RHCOS provisioning continues to work without changes | Regressions in existing deployment workflows |
| **BMO Integration** | BMO provisioning works without explicitly specifying deploy interface | BMO requires interface specification or fails |

---

## 8. Suspension Criteria and Resumption Requirements

### Suspension Criteria
Testing should be suspended if any of the following conditions occur:

1. **Critical Infrastructure Failure:** Test environment clusters or bare metal infrastructure unavailable
2. **Blocking Defects:**
   - Autodetect interface completely broken
   - Configuration changes not applied in deployed Ironic
   - All bootc deployments fail due to interface selection
   - Regressions block existing CoreOS provisioning
3. **Dependency Failures:** Ironic service unavailable or non-functional
4. **Configuration Errors:** Ironic configuration cannot be validated

### Resumption Requirements
Testing may resume when:

1. Infrastructure is restored and validated with smoke tests
2. Blocking defects are fixed and verified
3. Dependencies are restored
4. Ironic configuration is verifiable

---

## 9. Test Deliverables

1. **Test Plan Document:** This IEEE 829-style test plan (Markdown)
2. **Test Case Coverage Mapping:** Coverage summary with test case IDs, titles, and requirement traceability (see Section 17)
3. **Test Results Report:** Pass/fail status for each test case
4. **Defect Reports:** Jira issues for failures
5. **Configuration Verification:** Evidence of Ironic configuration changes
6. **CI Automation Artifacts:** Automated test scripts
7. **Regression Test Results:** Verification of existing provisioning methods

---

## 10. Testing Tasks

1. **Environment Setup** - Provision clusters with Metal3 and BMO, deploy Ironic with PR #911 changes, set up container registry with bootc images
2. **Test Case Development** - Author detailed specifications for all coverage categories
3. **Test Execution** - Execute functional, interface detection, bootc provisioning, fallback, and regression tests
4. **Results Analysis** - Analyze results, verify interface selection logging, document behavior
5. **Defect Management** - File and track Jira issues
6. **CI/CD Integration** - Integrate automated tests into pipeline

---

## 11. Environmental Needs

### Infrastructure Requirements

**Cluster Configurations:**
- OpenShift 5.1 clusters with Metal3 operator and BMO
- Ironic deployed with PR #911 configuration changes
- Container registry access (Quay, Harbor)

**Hardware Requirements:**
- Bare metal systems (BMC-enabled with IPMI/Redfish)
- Local storage for image deployment
- PXE boot capable network

**Container Images:**
- Bootc container images, plain OCI artifacts, LiveISO images, CoreOS/RHCOS images

---

## 12. Responsibilities

| Role | Testing Responsibilities |
|------|--------------------------|
| **QE Team** | Test plan development, test execution, configuration verification, results analysis, defect reporting, regression testing |
| **Development Team** | Implementation, defect fixes, code review, configuration validation |
| **SRE Team** | Test environment provisioning, infrastructure monitoring, registry management |
| **Release Engineering** | CI/CD pipeline integration, release criteria validation |
| **Product Management** | Requirements validation, feature scope approval |
| **Technical Writers** | Documentation of bootc provisioning workflows |

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
| **Configuration Not Applied** | Verify configuration in deployed Ironic; validate template rendering |
| **Bootc Image Availability** | Build test images early; use upstream images as fallback |
| **Autodetect Logic Issues** | Thoroughly test detection logic; validate against different image types |
| **Backward Compatibility Breaks** | Comprehensive regression testing; quick rollback plan |
| **Logging Insufficient** | Verify Ironic log verbosity; enhance if needed |

---

## 16. Approvals

This test plan requires approval from the following stakeholders:

- **QE Owner:** Steeve Goveas - Test execution and quality validation
- **Feature Owner:** _[Name/Role]_ - Requirements alignment and scope
- **Development Lead:** Dmitry Tantsur - Technical feasibility and implementation approach
- **Development Architect:** _[Name/Role]_ - Architecture and design review
- **Product Management:** Michal Zasepa - Business requirements
- **Release Engineering:** _[Name/Role]_ - Release readiness and CI/CD integration

**Approval Date:** TBD

---

## 17. Test Coverage Mapping

This section provides test coverage mapping with test case IDs, titles, and requirement traceability.

---

#### 17.1 Functional Validation

| Test Case ID | Title | Requirement Traceability | Coverage |
|--------------|-------|--------------------------|----------|
| TC-FUNC-01 | Verify autodetect is default deploy interface | FR-01, IF-01 | Complete |
| TC-FUNC-02 | Verify detection order: ramdisk, bootc, direct | FR-02, IF-02 | Complete |
| TC-FUNC-03 | Bootc OCI image selects bootc interface | FR-03 | Complete |
| TC-FUNC-04 | Plain OCI artifact selects direct interface | FR-04 | Complete |
| TC-FUNC-05 | Ramdisk interface has highest priority in autodetect order | FR-05, FR-02 | Complete |
| TC-FUNC-06 | Verify autodetect is in enabled interfaces | FR-01, IF-01 | Complete |
| TC-FUNC-07 | Interface selection logged in Ironic | IF-03 | Complete |
| TC-FUNC-08 | BMO provisioning without explicit interface | IF-02, INT-01 | Complete |
| TC-FUNC-09 | OCI index with missing descriptor metadata inspects child manifest for correct classification | FR-04 | Complete |
| TC-FUNC-10 | OCI index with missing descriptor metadata where child manifest is artifact selects direct interface | FR-04 | Complete |

---

#### 17.2 Testing and Validation

N/A - Covered by Functional Validation test cases above.

---

#### 17.3 Deployment/Topology Variations

**Note:** The autodetect feature is configuration-based and operates at the Ironic level, independent of cluster topology.

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-DEPLOY-01 | Autodetect interface on multi-node bare metal cluster | Complete |

---

#### 17.4 Interoperability

| Test Case ID | Title | Requirement Traceability | Coverage |
|--------------|-------|--------------------------|----------|
| TC-INTEROP-01 | BMO integration with Ironic autodetect interface | INT-01 | Complete |
| TC-INTEROP-02 | Coexistence with existing CoreOS provisioning | INT-02 | Complete |
| TC-INTEROP-03 | Interface detection with container registry | General | Complete |

---

#### 17.5 Upgrade/Rollback

**Note:** Rollback testing required due to Ironic node persistence. Nodes with `deploy_interface=autodetect` may fail after rollback if autodetect is removed from `enabled_deploy_interfaces`.

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-ROLLBACK-01 | Available node with autodetect survives rollback | Complete |
| TC-ROLLBACK-02 | Provisioned node undeploy-reprovision after rollback | Complete |

---

#### 17.6 Success Criteria

| Success Metric | Test Case ID | Pass Criteria | Coverage |
|----------------|--------------|---------------|----------|
| Autodetect configured correctly | TC-FUNC-01, TC-FUNC-06 | Configuration values match expected | Complete |
| Bootc images use bootc interface | TC-FUNC-03 | Bootc interface automatically selected | Complete |
| Fallback works for non-bootc images | TC-FUNC-04, TC-FUNC-09, TC-FUNC-10 | Direct interface selected correctly | Complete |
| LiveISO continues to work | TC-REG-02 | Ramdisk interface selected | Complete |

---

#### 17.7 Non-Functional Requirements

| NFR Category | Test Case ID | Title | Coverage |
|--------------|--------------|-------|----------|
| **Reliability** | TC-NFR-REL-01 | Consistent interface selection for same image type | Complete |
| **Reliability** | TC-NFR-REL-02 | Graceful handling when no interface matches | Complete |
| **Observability** | TC-NFR-OBS-01 | Interface selection visible in logs | Complete |
| **Performance** | TC-NFR-PERF-01 | Interface detection does not add significant delay | Complete |
| **Compatibility** | TC-NFR-COMPAT-01 | Works with current BMO version | Complete |

---

#### 17.8 Operational / Day-2

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-OPS-01 | Configuration verification procedure | Complete |
| TC-OPS-02 | Interface selection troubleshooting | Complete |
| TC-OPS-03 | Logging verification for debugging | Complete |

---

#### 17.9 Negative Tests

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-NEG-01 | Invalid image type handled gracefully | Complete |
| TC-NEG-02a | Configuration error: bootc in autodetect list but not enabled list | Complete |
| TC-NEG-02b | Bootc removed from both lists falls back to direct | Complete |
| TC-NEG-03 | Explicit interface specification still works | Complete |

---

#### 17.10 Regression Scenarios

| Test Case ID | Title | Coverage |
|--------------|-------|----------|
| TC-REG-01 | CoreOS/RHCOS provisioning still works | Complete |
| TC-REG-02 | LiveISO provisioning still works (ramdisk interface) | Complete |
| TC-REG-03 | Plain OCI artifact provisioning (direct fallback) | Complete |
| TC-REG-04 | BareMetalHost API backward compatibility | Complete |
| TC-REG-05 | BMO workflows unchanged for existing images | Complete |
| TC-REG-06 | Explicit interface specification still works | Complete |

---

## 18. Readiness Integration

- Link this test plan from OCPSTRAT-3103
- Store in Git repository: `openshift/cluster-baremetal-operator` under `test-plans/5.1/`
- Derive QE work items from Testing Tasks (Section 10) and Test Coverage Mapping (Section 17) with traceability to test case IDs

---

## 19. Notes and Limitations

### Implementation Scope
This test plan covers only the Ironic configuration changes in PR #911 that enable bootc deploy interface autodetection. It does not cover:
- Ironic Python Agent (IPA) bootc implementation (separate PR)
- Bootc image creation or management
- Advanced bootc features (SELinux, authentication, filesystem selection, OCI layout handling)

### Relationship to OCPSTRAT-3103
OCPSTRAT-3103 describes a broad BMaaS multi-OS provisioning vision (RHEL, Windows, etc.), while PR #911 implements a foundational piece - bootc deploy interface autodetection. This test plan focuses on the actual implementation rather than the broader vision. Future test plans will be needed as additional BMaaS capabilities are implemented.

### Future Enhancements
- IPA bootc provisioning implementation
- Multi-OS provisioning beyond bootc (RHEL, Windows)
- Multi-network provisioning
- Image management features

---

**Document Version:** 3.0  
**Last Updated:** 2026-11-09  
**Status:** Updated - PR #911 Only  
**Implementation Analyzed:** openshift/ironic-image#911
