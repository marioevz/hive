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
    - Epoch depends on whether builder workflow activation requires finalization for the CL client:
      - Lighthouse
      - Teku
  - Both nodes have the mock-builder configured as builder endpoint from the start
  - Total of 128 Validators, 64 for each validating node
  - Wait for Deneb fork
  - Verify that the builder was able to produce blocks and they have been included in the canonical chain
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
