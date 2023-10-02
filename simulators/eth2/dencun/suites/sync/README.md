# Dencun Interop Sync Simulator

To run locally use:

```bash
./hive --client-file ./configs/cancun.yaml --client "go-ethereum,${cl}-bn,${cl}-vc" --sim eth2/dencun --sim.limit "eth2-deneb-sync/"
```

## Test Cases

### Test Deneb Syncing

* [x] Sync From Capella Transition
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Capella/Shanghai genesis
  - Deneb/Cancun transition occurs on Epoch 1
  - Total of 128 Validators, 64 for each validating node
  - Wait for Deneb fork and start sending blob transactions to the Execution client
  - Wait one more epoch for the chain to progress and include blobs
  - Start a third client with the first two validating nodes as bootnodes
  - Wait for the third client to reach the head of the canonical chain
  - Verify on the consensus client on the synced client that:
    - For each blob transaction on the execution chain, the blob sidecars are available for the
      beacon block at the same height
    - The beacon block lists the correct commitments for each blob
    - The blob sidecars and kzg commitments match on each block for the synced client
  
  </details>

* [x] Sync From Deneb Genesis
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that begin on Deneb genesis
  - Total of 128 Validators, 64 for each validating node
  - Start sending blob transactions to the Execution client
  - Wait one epoch for the chain to progress and include blobs
  - Start a third client with the first two validating nodes as bootnodes
  - Wait for the third client to reach the head of the canonical chain
  - Verify on the consensus client on the synced client that:
    - For each blob transaction on the execution chain, the blob sidecars are available for the
      beacon block at the same height
    - The beacon block lists the correct commitments for each blob
    - The blob sidecars and kzg commitments match on each block for the synced client
  
  </details>

