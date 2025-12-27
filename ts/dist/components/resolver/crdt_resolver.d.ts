import { DistributedDocument, CRDTOperation } from '../types/types';
export declare function ResolveConflict(local: DistributedDocument | null, remote: DistributedDocument | null): DistributedDocument | null;
export declare function ApplyOperation(doc: DistributedDocument | null, op: CRDTOperation): DistributedDocument | null;
export declare function ToDistributed(payload: Record<string, any>, peerID: string): DistributedDocument;
export declare function ToRegular(doc: DistributedDocument | null): Record<string, any> | null;
//# sourceMappingURL=crdt_resolver.d.ts.map