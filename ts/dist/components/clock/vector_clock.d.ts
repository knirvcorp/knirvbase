export type VectorClock = Record<string, number>;
export declare enum ComparisonResult {
    Equal = 0,
    Before = 1,
    After = 2,
    Concurrent = 3
}
export declare function increment(clock: VectorClock, peerID: string): VectorClock;
export declare function merge(clock1: VectorClock, clock2: VectorClock): VectorClock;
export declare function compare(clock1: VectorClock, clock2: VectorClock): ComparisonResult;
export declare function happensBefore(clock1: VectorClock, clock2: VectorClock): boolean;
export declare function newVectorClock(): VectorClock;
export declare function clone(clock: VectorClock): VectorClock;
//# sourceMappingURL=vector_clock.d.ts.map