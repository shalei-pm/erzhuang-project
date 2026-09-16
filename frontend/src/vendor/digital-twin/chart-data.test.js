import { expect, it } from 'vitest';
import chartData from './chart-data.js';

it('renders visit totals until all three arrival paths become available', () => {
 const visits = chartData.chartSpecs.find(spec => spec.id === 'visits');
 expect(visits.type).toBe('stacked');
 expect(visits.series).toEqual([
  { label: '全部顾客', key: 'visitAll', color: '#e3e9d9' },
  { label: '前台签到', key: 'frontDesk', color: '#74a5ff' },
  { label: '非面诊', key: 'noConsult', color: '#00dca0' },
  { label: '面诊', key: 'consult', color: '#ff913d' },
 ]);
 expect(chartData.seriesForDatum(visits, {visitAll:55,frontDesk:null,noConsult:34,consult:18}).map(series => series.key)).toEqual(['visitAll']);
 expect(chartData.seriesForDatum(visits, {visitAll:55,frontDesk:3,noConsult:34,consult:18}).map(series => series.key)).toEqual(['frontDesk','noConsult','consult']);
});

it('uses the approved BI titles, units, and audience labels', () => {
 const specs = Object.fromEntries(chartData.chartSpecs.map(spec => [spec.id, spec]));
 expect([specs.visits.title, specs.visits.unit]).toEqual(['到院人次', '人次']);
 expect(specs.stay.title).toBe('在店时长');
 expect(specs.upgrade.title).toBe('升单率');
 expect([specs.redemption.title, specs.redemption.unit]).toEqual(['核销客单价', '元/人次']);
 expect([specs['service-points'].title, specs['service-points'].unit]).toEqual(['人均服务点数', '点/人次']);
 for (const spec of chartData.chartSpecs.filter(spec => spec.id !== 'visits')) {
  expect(spec.series.map(series => series.label)).toEqual(['全部顾客', '非面诊', '面诊']);
 }
});
