# Dencun Interop Builder Simulator

To run locally use:

```bash
./hive --client-file ./configs/cancun.yaml --client "go-ethereum,${cl}-bn,${cl}-vc" --sim eth2/dencun --sim.limit "eth2-deneb-builder/"
```

## Test Cases

### Test Deneb Builder Correct Workflow

* [x] Deneb Builder Workflow From Capella Transition
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Capella/Shanghai genesis
  - Deneb/Cancun transition occurs on Epoch 1 or 5
    - Epoch depends on whether builder workflow activation requires finalization for the CL client. Following clients require finalization:
      - Lighthouse
      - Teku
  - Both nodes have the mock-builder configured as builder endpoint from the start
  - Total of 128 Validators, 64 for each validating node
  - Wait for Deneb fork
  - Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
  - Start sending blob transactions to the Execution client
  - Wait one more epoch for the chain to progress and include blobs
  - Verify on the beacon chain that:
    - Builder was able to include blocks with blobs in the canonical chain, which implicitly verifies:
      - Consensus client is able to properly format header requests to the builder
      - Consensus client is able to properly format blinded signed requests to the builder
    - No signed block or blob sidecar contained an invalid format or signature
    - For each blob transaction on the execution chain, the blob sidecars are available for the
      beacon block at the same height
    - The beacon block lists the correct commitments for each blob
    - Chain is finalizing
    - No more than two missed slots on the latest epoch
  
  </details>

### Test Deneb Builder Error Workflow

* [x] Deneb Builder Builds Block With Invalid Beacon Root, Correct State Root
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Capella/Shanghai genesis
  - Deneb/Cancun transition occurs on Epoch 1 or 5
    - Epoch depends on whether builder workflow activation requires finalization for the CL client. Following clients require finalization:
      - Lighthouse
      - Teku
  - Both nodes have the mock-builder configured as builder endpoint from the start
  - Total of 128 Validators, 64 for each validating node
  - Wait for Deneb fork
  - Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
  - Start sending blob transactions to the Execution client
  - Starting from Deneb, Mock builder starts corrupting the payload attributes' parent beacon block root sent to the execution client to produce the payloads
  - Wait one more epoch for the chain to progress
  - Verify on the beacon chain that:
    - Blocks with the corrupted beacon root are not included in the canonical chain
    - Circuit breaker correctly kicks in and disables the builder workflow

  </details>

* [x] Deneb Builder Errors Out on Header Requests
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Capella/Shanghai genesis
  - Deneb/Cancun transition occurs on Epoch 1 or 5
    - Epoch depends on whether builder workflow activation requires finalization for the CL client. Following clients require finalization:
      - Lighthouse
      - Teku
  - Both nodes have the mock-builder configured as builder endpoint from the start
  - Total of 128 Validators, 64 for each validating node
  - Wait for Deneb fork
  - Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
  - Start sending blob transactions to the Execution client
  - Starting from Deneb, Mock builder starts returning error on the request for block headers
  - Wait one more epoch for the chain to progress
  - Verify on the beacon chain that:
    - Consensus clients fallback to local block building
    - No more than two missed slots on the latest epoch

  </details>

* [x] Deneb Builder Errors Out on Signed Blinded Beacon Block/Blob Sidecars Submission
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Capella/Shanghai genesis
  - Deneb/Cancun transition occurs on Epoch 1 or 5
    - Epoch depends on whether builder workflow activation requires finalization for the CL client. Following clients require finalization:
      - Lighthouse
      - Teku
  - Both nodes have the mock-builder configured as builder endpoint from the start
  - Total of 128 Validators, 64 for each validating node
  - Wait for Deneb fork
  - Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
  - Start sending blob transactions to the Execution client
  - Starting from Deneb, Mock builder starts returning error on the submission of signed blinded beacon block/blob sidecars
  - Wait one more epoch for the chain to progress
  - Verify on the beacon chain that:
    - Signed missed slots do not fallback to local block building
    - Circuit breaker correctly kicks in and disables the builder workflow

  </details>


* [x] Deneb Builder Builds Block With Invalid Beacon Root, Incorrect State Root
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Capella/Shanghai genesis
  - Deneb/Cancun transition occurs on Epoch 1 or 5
    - Epoch depends on whether builder workflow activation requires finalization for the CL client. Following clients require finalization:
      - Lighthouse
      - Teku
  - Both nodes have the mock-builder configured as builder endpoint from the start
  - Total of 128 Validators, 64 for each validating node
  - Wait for Deneb fork
  - Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
  - Start sending blob transactions to the Execution client
  - Starting from Deneb, Mock builder starts corrupting the parent beacon block root of the payload received from the execution client
  - Wait one more epoch for the chain to progress
  - Verify on the beacon chain that:
    - Blocks with the corrupted beacon root are not included in the canonical chain
    - Circuit breaker correctly kicks in and disables the builder workflow

  </details>