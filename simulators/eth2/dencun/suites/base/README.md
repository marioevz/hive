# Dencun Interop Testnet Simulator

To run locally use:

```bash
./hive --client-file ./configs/cancun.yaml --client "go-ethereum,${cl}-bn,${cl}-vc" --sim eth2/dencun --sim.limit "eth2-deneb-testnet/"
```

## Test Cases

### Deneb/Cancun Transition

* [x] Deneb/Cancun Transition
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

* [x] Deneb/Cancun Genesis
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

