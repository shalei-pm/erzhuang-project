import { expect, it } from 'vitest';
import chartData from './chart-data.js';

it('renders the 30-day visits trend as a single-series bar chart', () => {
 const visits = chartData.chartSpecs.find(spec => spec.id === 'visits');
 expect(visits.type).toBe('stacked');
 expect(visits.series).toEqual([{ label: '全部顾客', key: 'visitAll', color: '#e3e9d9' }]);
});
