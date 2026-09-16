import { expect, it } from 'vitest';
import chartData from './chart-data.js';

it('renders visit totals until segmented visit data becomes available', () => {
 const visits = chartData.chartSpecs.find(spec => spec.id === 'visits');
 expect(visits.type).toBe('stacked');
 expect(visits.series).toEqual([
  { label: '全部顾客', key: 'visitAll', color: '#e3e9d9' },
  { label: '无需咨询', key: 'noConsult', color: '#00dca0' },
  { label: '需要面诊', key: 'consult', color: '#ff913d' },
 ]);
 expect(chartData.seriesForDatum(visits, {visitAll:42,noConsult:null,consult:null}).map(series => series.key)).toEqual(['visitAll']);
 expect(chartData.seriesForDatum(visits, {visitAll:42,noConsult:27,consult:15}).map(series => series.key)).toEqual(['noConsult','consult']);
});
