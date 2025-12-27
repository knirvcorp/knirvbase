import { increment, merge, compare, ComparisonResult, happensBefore, newVectorClock, clone } from './vector_clock';

describe('VectorClock', () => {
  describe('increment', () => {
    it('should increment existing peer counter', () => {
      const clock = { a: 1, b: 2 };
      const result = increment(clock, 'a');
      expect(result.a).toBe(2);
      expect(result.b).toBe(2);
    });

    it('should add new peer counter', () => {
      const clock = { a: 1 };
      const result = increment(clock, 'b');
      expect(result.a).toBe(1);
      expect(result.b).toBe(1);
    });

    it('should handle null clock', () => {
      const result = increment(null as any, 'a');
      expect(result.a).toBe(1);
    });
  });

  describe('merge', () => {
    it('should merge two clocks taking max values', () => {
      const clock1 = { a: 1, b: 2 };
      const clock2 = { a: 3, c: 4 };
      const result = merge(clock1, clock2);
      expect(result).toEqual({ a: 3, b: 2, c: 4 });
    });
  });

  describe('compare', () => {
    it('should return Equal for identical clocks', () => {
      const clock1 = { a: 1, b: 2 };
      const clock2 = { a: 1, b: 2 };
      expect(compare(clock1, clock2)).toBe(ComparisonResult.Equal);
    });

    it('should return After when first clock is ahead', () => {
      const clock1 = { a: 2, b: 2 };
      const clock2 = { a: 1, b: 2 };
      expect(compare(clock1, clock2)).toBe(ComparisonResult.After);
    });

    it('should return Before when first clock is behind', () => {
      const clock1 = { a: 1, b: 2 };
      const clock2 = { a: 2, b: 2 };
      expect(compare(clock1, clock2)).toBe(ComparisonResult.Before);
    });

    it('should return Concurrent for concurrent clocks', () => {
      const clock1 = { a: 2, b: 1 };
      const clock2 = { a: 1, b: 2 };
      expect(compare(clock1, clock2)).toBe(ComparisonResult.Concurrent);
    });
  });

  describe('happensBefore', () => {
    it('should return true for equal clocks', () => {
      const clock1 = { a: 1, b: 2 };
      const clock2 = { a: 1, b: 2 };
      expect(happensBefore(clock1, clock2)).toBe(true);
    });

    it('should return true for before clocks', () => {
      const clock1 = { a: 1, b: 2 };
      const clock2 = { a: 2, b: 2 };
      expect(happensBefore(clock1, clock2)).toBe(true);
    });

    it('should return false for after clocks', () => {
      const clock1 = { a: 2, b: 2 };
      const clock2 = { a: 1, b: 2 };
      expect(happensBefore(clock1, clock2)).toBe(false);
    });
  });

  describe('newVectorClock', () => {
    it('should return empty object', () => {
      const clock = newVectorClock();
      expect(clock).toEqual({});
    });
  });

  describe('clone', () => {
    it('should return shallow copy', () => {
      const original = { a: 1, b: 2 };
      const cloned = clone(original);
      expect(cloned).toEqual(original);
      expect(cloned).not.toBe(original); // Different reference
    });

    it('should handle null clock', () => {
      const cloned = clone(null as any);
      expect(cloned).toEqual({});
    });
  });
});