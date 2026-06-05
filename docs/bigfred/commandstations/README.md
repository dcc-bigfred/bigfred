# Command stations (centralki)

Hardware reference for physical **command stations** (*centralki*) supported or
commonly used with BigFred. For how BigFred wires a station into the hub
(catalogue row, `dcc-bus` daemon, layout attachment), see
[`16-dcc-bus`](../architecture/16-dcc-bus/README.md) and the
[`loconet-adapter`](../../loconet-adapter/README.md) bring-up guides.

| Model | Manufacturer | BigFred connection | Document |
|-------|--------------|-------------------|----------|
| **RB1110** / **RB1110-Mini** | [RailBOX Electronics](https://www.railbox.pl/) | **`z21`** UDP on LAN/WiFi (recommended) | [RB1110](./rb1110.md) |
| **Digikeijs DR5000** | [Digikeijs](https://www.digikeijs.com/) | **`loconet_serial`** + Uhlenbrock 63120 on **LocoNet-T** | [DR5000](./dr5000.md) |

Terminology: [`00-terminology.md`](../architecture/00-terminology.md) (*command
station* / *centralka*).
