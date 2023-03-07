# Withdrawals Interop Hive Simulator

The simulator contains the implementation of the tests described in this
document.


## General Considerations for all tests
- A single validating node comprises an execution client, beacon client and validator client (unless specified otherwise)
- All validating nodes have the same number of validators (unless specified otherwise)
- An importer node is a node that has no validators but might be connected to the network
- Execution client Shanghai timestamp transition is automatically calculated from the Capella Epoch timestamp


## Test Cases

### Capella/Shanghai Transition

* [x] Capella/Shanghai Transition
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Bellatrix/Merge genesis
  - Capella/Shanghai transition occurs on Epoch 1
  - Total of 128 Validators, 64 for each validating node
  - All validators contain 0x01 withdrawal credentials
  - Wait for Capella fork + `128 / MAX_WITHDRAWALS_PER_PAYLOAD` slots
  - Verify on the execution client:
    - All active validators' balances increase
  
  </details>

### Capella/Shanghai Genesis

* [x] Capella/Shanghai Genesis
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Capella genesis
  - Total of 128 Validators, 64 for each validating node
  - All validators contain 0x01 withdrawal credentials
  - Wait `128 / MAX_WITHDRAWALS_PER_PAYLOAD` slots
  - Verify on the execution client:
    - All active validators' balances increase
  
  </details>

### BLS-To-Execution-Change

* [x] BLS-To-Execution-Changes
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Bellatrix/Merge genesis
  - Capella/Shanghai transition occurs on Epoch 1
  - Half genesis validators have BLS withdrawal credentials
  - If any of the clients supports receiving BLS-To-Execution-Changes during Bellatrix, sign and submit half of the BLS validators during the first epoch.
  - Wait for Capella fork
  - Submit the remaining BLS-To-Execution-Changes to all nodes
  - Wait and verify on the beacon state that withdrawal credentials are updated
  - Verify on the execution client:
    - All active validators' balances increase

  * [x] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes before transition to Capella/Shanghai
  * [x] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes after transition to Capella/Shanghai

  </details>

* [x] BLS-To-Execution-Changes of Exited/Slashed Validators
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Bellatrix/Merge genesis
  - Capella/Shanghai transition occurs on Epoch 1
  - Total of 128 Validators, 64 for each validating node
  - Half genesis validators have BLS withdrawal credentials
  - 1 every 8 validators start on Voluntary Exit state
  - 1 every 8 validators start on Slashed state
  - If any of the clients supports receiving BLS-To-Execution-Changes during Bellatrix, sign and submit half of the BLS validators during the first epoch.
  - Wait for Capella fork
  - Submit the remaining BLS-To-Execution-Changes to all nodes
  - Wait and verify on the beacon state that withdrawal credentials are updated
  - Verify on the beacon state:
    - Withdrawal credentials are updated
    - Fully exited validators' balances drop to zero
  - Verify on the execution client:
    - All active validators' balances increase
    - Fully exited validators' balances are equal to the full withdrawn amount

  * [x] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes before transition to Capella/Shanghai
  * [x] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes after transition to Capella/Shanghai

  

  </details>

* [ ] BLS-To-Execution-Changes Broadcast
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes and one importer node on Capella/Shanghai genesis
  - All genesis validators have BLS withdrawal credentials
  - Sign and submit BLS-To-Execution-Changes to the importer node of all validators to change withdrawal credentials to different execution addresses
  - Wait until the importer node broadcasts the BLS-To-Execution-Changes
  - Verify on the beacon state:
    - Withdrawal credentials are updated
  - Verify on the execution client:
    - All active validators' balances increase

  * [ ] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes before transition to Capella/Shanghai
  * [ ] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes after transition to Capella/Shanghai

  </details>

### Withdrawals During Re-Orgs

* [X] Full/Partial Withdrawals and BLS Changes on Re-Org
  <details>
  <summary>Click for details</summary>
  
  - Start four validating nodes on Capella/Shanghai genesis
  - Two pairs of nodes, `A` (`A1` + `A2`) and `B` (`B1` + `B2`), are connected only between each other.
  - Total of 128 Validators, 42 for each validating node
  - All genesis validators have BLS withdrawal credentials
  - On Epoch 0, submit BLS-To-Execution changes to each pair `A` and `B`, but the execution addresses are different on each pair.
  - Wait for all the withdrawals credentials of all validators to be updated on both pairs
  - Wait for all validators to fully or partially withdraw on both pairs
  - Start a fifth node `C1` that connects to all nodes `A` and `B`
  - Wait until all nodes converge into a canonical head
  - Verify that:
    - BLS-To-Execution changes of one of the pair's chain become canonical
    - Withdrawals of one of the pair's chain become canonical
    - Execution addresses of the canonical pair's chain have the correct fully or partial amounts
    - Execution addresses of the orphaned chain are empty

  </details>

### Builder API Fallback for Withdrawals

* [x] Builder API Constructs Payloads with Invalid Withdrawals List
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Bellatrix/Paris genesis
  - Total of 128 Validators, 64 for each validating node
  - All genesis validators have Execution address withdrawal credentials
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to return payloads with an invalid withdrawals list, starting from capella
  - Wait for finalization, and verify at least one block was built by the builder API on each node
  - Wait for capella and verify that the invalid payloads are correctly rejected from the canonical chain
  - Verify that the chain is able to finalize even after the builder API returns payloads with invalid withdrawals on every request

  </details>

* [x] Builder API Returns Error on Header Request Starting from Capella
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Bellatrix/Paris genesis
  - Total of 128 Validators, 64 for each validating node
  - All genesis validators have Execution address withdrawal credentials
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to return error on header request, starting from capella
  - Wait for capella
  - Wait for finalization, and verify at least one block was built by the builder API on each node
  - Verify that the chain is able to finalize even after the builder API returns error on every header request

  </details>

* [x] Builder API Returns Error on Unblinded Block Request Starting from Capella
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Bellatrix/Paris genesis
  - Total of 128 Validators, 64 for each validating node
  - All genesis validators have Execution address withdrawal credentials
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to return error on unblinded block request, starting from capella
  - Wait for capella
  - Wait for finalization, and verify at least one block was built by the builder API on each node
  - Verify that the chain is able to finalize even after the builder API returns error on every unblinded block request

  </details>

* [x] Builder API Returns Constructs Valid Withdrawals/Invalid StateRoot Payload Starting from Capella
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Bellatrix/Paris genesis
  - Total of 128 Validators, 64 for each validating node
  - All genesis validators have Execution address withdrawal credentials
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to produce payloads with valid withdrawals list, but invalid state root, starting from capella
  - Wait for capella
  - Verify that the consensus clients correctly circuit break the builder when the empty slots are detected
  - Verify that the chain is able to finalize

  </details>