/**
 * TypeScript mirrors of EdgeVPN's Go wire types.
 *
 * Field names are PascalCase on purpose: the Go structs in pkg/types and
 * api/types carry no `json` struct tags, so encoding/json emits the Go
 * field names verbatim. Adding tags would break api/client and external
 * consumers (Kairos, LocalAI), so the casing is absorbed here instead.
 */

/** pkg/types.Summary */
export interface Summary {
  Files: number
  Machines: number
  Users: number
  Services: number
  BlockChain: number
  OnChainNodes: number
  Peers: number
  NodeID: string
}

/** pkg/types.Machine, embedded in api/types.Machine */
export interface Machine {
  PeerID: string
  Hostname: string
  OS: string
  Arch: string
  Address: string
  Version: string
  Connected: boolean
  OnChain: boolean
  Online: boolean
}

/** api/types.Peer. Note: /api/peerstore always reports Online === false. */
export interface Peer {
  ID: string
  Online: boolean
}

/** pkg/types.User */
export interface User {
  PeerID: string
  Timestamp: string
}

/** pkg/types.Service */
export interface Service {
  PeerID: string
  Name: string
}

/** pkg/types.File — named FileEntry to avoid clashing with the DOM File type. */
export interface FileEntry {
  PeerID: string
  Name: string
}

/** api/types.DNS */
export interface DNSEntry {
  Regex: string
  Records: Record<string, string>
}

/** libp2p metrics.Stats */
export interface Stats {
  TotalIn: number
  TotalOut: number
  RateIn: number
  RateOut: number
}

/** GET /api/metrics/peer — keyed by peer ID */
export type PeerStats = Record<string, Stats>

/** blockchain.Block */
export interface Block {
  Index: number
  Timestamp: string
  Hash: string
  PrevHash: string
  Storage: Record<string, Record<string, unknown>>
}
