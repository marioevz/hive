# Dencun Interop Hive Simulator

The simulator contains implementation of tests that verify interoperability behavior of the
Execution and Consensus clients on the Deneb+Cancun (Dencun) hard-fork.


## General Considerations for all tests
- A single validating node comprises an execution client, beacon client and validator client (unless specified otherwise)
- All validating nodes have the same number of validators (unless specified otherwise)
- An importer node is a node that has no validators but might be connected to the network
- Execution client Cancun timestamp transition is automatically calculated from the Deneb Epoch timestamp


## Test Cases

### Deneb/Cancun Transition

* [ ] Deneb/Cancun Transition
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Capella/Shanghai genesis
  - Deneb/Cancun transition occurs on Epoch 1
  - Total of 128 Validators, 64 for each validating node
  - Wait for Deneb fork and start sending blob transactions to the Execution client
  - Verify on the execution client that:
    - Blob (type-3) transactions are included in the blocks
  - Verify on the consensus client that:
    - For each blob transaction on the execution chain, the blob sidecars are available for the
      beacon block at the same height
    - The beacon block lists the correct commitments for each blob
  
  </details>

### Deneb/Cancun Genesis

* [ ] Deneb/Cancun Genesis
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Deneb genesis
  - Total of 128 Validators, 64 for each validating node
  - From the beginning send blob transactions to the Execution client
  - Verify on the execution client that:
    - Blob (type-3) transactions are included in the blocks
  - Verify on the consensus client that:
    - For each blob transaction on the execution chain, the blob sidecars are available for the
      beacon block at the same height
    - The beacon block lists the correct commitments for each blob
  
  </details>


### Blobs During Re-Orgs

* TBD


### Builder API for Deneb

* [x] Builder API Constructs Payloads with Valid Blobs
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Capella/Shanghai genesis
  - Total of 128 Validators, 64 for each validating node
  - All genesis validators have BLS withdrawal credentials
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to return payloads with the correct withdrawals list, and correct blobs, starting from Deneb
  - Wait for Deneb and start sending blob transactions to the execution client, also start sending BLS-to-execution-changes
  - Verify that the payloads are correctly included in the canonical chain
  - Wait for finalization, and verify at least one block was built by the builder API on each node
  - Verify that all signed beacon blocks delivered to the builder were correctly constructed and signed
  - Verify that at least one block built by the mock builder contained blobs, and that the blinded blob sidecar signed by the consensus client was correctly formatted and signed

  </details>

* [x] Builder API Constructs Payloads with Corrupted Parent Beacon Block Root 
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Capella/Shanghai genesis
  - Total of 128 Validators, 64 for each validating node
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to return payloads with an invalid parent beacon block root, starting from Deneb
  - Wait for finalization, and verify at least one block was built by the builder API on each node
  - Wait for Deneb and verify that the invalid payloads are correctly rejected from the canonical chain
  - Verify that the chain is able to finalize even after the builder API returns payloads with invalid parent beacon block root on every request

  </details>

* [x] Builder API Returns Error on Header Request Starting from Deneb
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Capella/Shanghai genesis
  - Total of 128 Validators, 64 for each validating node
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to return error on header request, starting from Deneb
  - Wait for Deneb
  - Wait for finalization, and verify at least one block was requested to the builder API on each node
  - Verify that the chain is able to finalize even after the builder API returns error on every header request
  - Verify there are no missed slots on the chain

  </details>

* [x] Builder API Returns Error on Unblinded Block Request Starting from Deneb
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Capella/Shanghai genesis
  - Total of 128 Validators, 64 for each validating node
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to return error on unblinded block request, starting from Deneb
  - Wait for Deneb
  - Wait for finalization, and verify at least one block was built by the builder API on each node
  - Verify that the chain is able to finalize even after the builder API returns error on every unblinded block request (due to the circuit breaker kicking in)

  </details>

* [x] Builder API Returns Constructs Invalid Parent Beacon Block Root Payload Starting from Deneb
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Capella/Shanghai genesis
  - Total of 128 Validators, 64 for each validating node
  - Both validating nodes are connected to a builder API mock server
  - Builder API server is configured to produce payloads with an invalid parent beacon block root, starting from Deneb
  - Wait for Deneb
  - Verify that the consensus clients correctly circuit break the builder when the empty slots are detected
  - Verify that the chain is able to finalize

  </details>