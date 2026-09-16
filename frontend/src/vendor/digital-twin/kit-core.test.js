import { expect, it } from 'vitest';
import core from './kit-core.js';

it('accepts front desk arrivals as an optional trend count', () => {
 const current=core.emptyData();
 const next=core.merge(current,{trends:[{date:'2026-08-25',visitAll:55,frontDesk:3,noConsult:34,consult:18}]});
 expect(next.trends[0]).toMatchObject({visitAll:55,frontDesk:3,noConsult:34,consult:18});
});
