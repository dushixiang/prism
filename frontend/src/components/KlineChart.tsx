import {useEffect, useRef} from 'react';
import type {IChartApi, ISeriesApi, UTCTimestamp} from 'lightweight-charts';
import {CandlestickSeries, ColorType, createChart, HistogramSeries, LineSeries} from 'lightweight-charts';
import type {KlineData, TechnicalIndicators} from '@/types';
import { BarChart3 } from 'lucide-react';

interface KlineChartProps {
    data: KlineData[];
    indicators?: TechnicalIndicators;
    symbol?: string;
    height?: number;
}

export function KlineChart({data, indicators, symbol = '', height = 500}: KlineChartProps) {
    const chartContainerRef = useRef<HTMLDivElement>(null);
    const chartRef = useRef<IChartApi | null>(null);

    useEffect(() => {
        if (!chartContainerRef.current || !data.length) return;

        // 创建图表
        const chart = createChart(chartContainerRef.current, {
            layout: {
                background: {type: ColorType.Solid, color: '#ffffff'},
                textColor: '#333',
            },
            width: chartContainerRef.current.clientWidth,
            height: height,
            grid: {
                vertLines: {color: '#f0f0f0'},
                horzLines: {color: '#f0f0f0'},
            },
            crosshair: { mode: 1 },
            rightPriceScale: { borderColor: '#cccccc' },
            timeScale: { borderColor: '#cccccc', timeVisible: true, secondsVisible: false },
        });

        chartRef.current = chart;

        // 主图收出区域缩放：保留下方40%给MACD和KDJ
        chart.priceScale('right').applyOptions({
            scaleMargins: { top: 0, bottom: 0.4 },
        });

        // 添加K线系列 - v5 API (绿涨红跌)
        const candlestickSeries = chart.addSeries(CandlestickSeries, {
            upColor: '#22c55e',
            downColor: '#ef4444',
            borderDownColor: '#ef4444',
            borderUpColor: '#22c55e',
            wickDownColor: '#ef4444',
            wickUpColor: '#22c55e',
        });

        // 添加MA系列（覆盖在主图）
        const ma5Series = chart.addSeries(LineSeries, { color: 'rgba(31, 119, 180, 0.6)', lineWidth: 1, title: 'MA5' });
        const ma20Series = chart.addSeries(LineSeries, { color: 'rgba(255, 127, 14, 0.6)', lineWidth: 1, title: 'MA20' });
        const ma50Series = chart.addSeries(LineSeries, { color: 'rgba(44, 160, 44, 0.6)', lineWidth: 1, title: 'MA50' });

        // 布林带（覆盖在主图）
        let bollBandsUpper: ISeriesApi<'Line'> | null = null;
        let bollBandsLower: ISeriesApi<'Line'> | null = null;
        let bollBandsMid: ISeriesApi<'Line'> | null = null;
        if (indicators?.bb_upper && indicators?.bb_lower && indicators?.bb_middle) {
            bollBandsUpper = chart.addSeries(LineSeries, { color: 'rgba(156, 39, 176, 0.4)', lineWidth: 1, lineStyle: 2, title: 'BB Upper' });
            bollBandsLower = chart.addSeries(LineSeries, { color: 'rgba(156, 39, 176, 0.4)', lineWidth: 1, lineStyle: 2, title: 'BB Lower' });
            bollBandsMid = chart.addSeries(LineSeries, { color: 'rgba(156, 39, 176, 0.5)', lineWidth: 1, title: 'BB Mid' });
        }

        // 先创建 MACD 与 KDJ 系列，再设置对应 priceScale 选项
        const macdHist = chart.addSeries(HistogramSeries, { priceScaleId: 'macd' });
        const macdLine = chart.addSeries(LineSeries, { priceScaleId: 'macd', color: '#2563eb', lineWidth: 1 });
        const macdSignal = chart.addSeries(LineSeries, { priceScaleId: 'macd', color: '#f59e0b', lineWidth: 1 });
        chart.priceScale('macd').applyOptions({ scaleMargins: { top: 0.6, bottom: 0.2 } });

        const kLine = chart.addSeries(LineSeries, { priceScaleId: 'kdj', color: '#2563eb', lineWidth: 1 });
        const dLine = chart.addSeries(LineSeries, { priceScaleId: 'kdj', color: '#f59e0b', lineWidth: 1 });
        const jLine = chart.addSeries(LineSeries, { priceScaleId: 'kdj', color: '#7c3aed', lineWidth: 1 });
        chart.priceScale('kdj').applyOptions({ scaleMargins: { top: 0.8, bottom: 0 } });

        // 数据映射
        const candlestickData = data.map(item => ({
            time: (new Date(item.open_time).getTime() / 1000) as UTCTimestamp,
            open: item.open_price,
            high: item.high_price,
            low: item.low_price,
            close: item.close_price,
        }));
        candlestickSeries.setData(candlestickData);

        // MA 数据
        const toTs = (idx:number) => (new Date(data[idx].open_time).getTime() / 1000) as UTCTimestamp;
        const avg = (arr:number[]) => arr.reduce((a,b)=>a+b,0)/arr.length;
        const buildMA = (period:number) => data.map((_, i) => {
            if (i < period-1) return null;
            const slice = data.slice(i-(period-1), i+1).map(d=>d.close_price);
            return { time: toTs(i), value: avg(slice) };
        }).filter(Boolean) as {time:UTCTimestamp, value:number}[];
        ma5Series.setData(buildMA(5));
        ma20Series.setData(buildMA(20));
        ma50Series.setData(buildMA(50));

        // 布林带
        if (bollBandsUpper && bollBandsLower && bollBandsMid) {
            const bb = data.map((_, i) => {
                if (i < 19) return null;
                const slice = data.slice(i-19, i+1).map(d=>d.close_price);
                const m = avg(slice);
                const variance = slice.reduce((acc, p)=>acc + Math.pow(p-m,2), 0) / 20;
                const stdDev = Math.sqrt(variance);
                return { time: toTs(i), upper: m + 2*stdDev, middle: m, lower: m - 2*stdDev };
            }).filter(Boolean) as {time:UTCTimestamp, upper:number, middle:number, lower:number}[];
            bollBandsUpper.setData(bb.map(x=>({time:x.time, value:x.upper})));
            bollBandsLower.setData(bb.map(x=>({time:x.time, value:x.lower})));
            bollBandsMid.setData(bb.map(x=>({time:x.time, value:x.middle})));
        }

        // 计算 MACD
        const closes = data.map(d=>d.close_price);
        const ema = (arr:number[], period:number) => {
            const k = 2 / (period + 1);
            const out:number[] = [];
            let prev = arr[0];
            out.push(prev);
            for (let i=1;i<arr.length;i++){ prev = arr[i]*k + prev*(1-k); out.push(prev); }
            return out;
        };
        const ema12 = ema(closes, 12);
        const ema26 = ema(closes, 26);
        const macd = ema12.map((v,i)=> v - ema26[i]);
        const signalArr = ema(macd, 9);
        const hist = macd.map((v,i)=> v - signalArr[i]);
        macdHist.setData(hist.map((v, i) => ({ time: toTs(i), value: v, color: v >= 0 ? '#16a34a99' : '#dc262699' })));
        macdLine.setData(macd.map((v, i)=> ({ time: toTs(i), value: v })));
        macdSignal.setData(signalArr.map((v, i)=> ({ time: toTs(i), value: v })));

        // 计算 KDJ
        const highs = data.map(d=>d.high_price);
        const lows = data.map(d=>d.low_price);
        const period = 9;
        let kPrev = 50, dPrev = 50;
        const kVals:number[] = [], dVals:number[] = [], jVals:number[] = [];
        for (let i=0;i<closes.length;i++){
            const start = Math.max(0, i - period + 1);
            const hh = Math.max(...highs.slice(start, i+1));
            const ll = Math.min(...lows.slice(start, i+1));
            const rsv = hh === ll ? 50 : (closes[i]-ll)/(hh-ll)*100;
            const k = (2/3)*kPrev + (1/3)*rsv;
            const d = (2/3)*dPrev + (1/3)*k;
            kVals.push(k); dVals.push(d); jVals.push(3*k - 2*d);
            kPrev = k; dPrev = d;
        }
        kLine.setData(kVals.map((v,i)=> ({ time: toTs(i), value: v })));
        dLine.setData(dVals.map((v,i)=> ({ time: toTs(i), value: v })));
        jLine.setData(jVals.map((v,i)=> ({ time: toTs(i), value: v })));

        // 自适应内容
        chart.timeScale().fitContent();

        // 响应式处理
        const handleResize = () => {
            if (chartContainerRef.current) {
                chart.applyOptions({ width: chartContainerRef.current.clientWidth, height: height });
            }
        };
        window.addEventListener('resize', handleResize);

        return () => {
            window.removeEventListener('resize', handleResize);
            chart.remove();
        };
    }, [data, indicators, height]);

    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <h3 className="text-lg font-semibold flex items-center">
                    <BarChart3 className="w-5 h-5 mr-2" />
                    {symbol} K线图表
                </h3>
                <div className="flex items-center space-x-4 text-sm">
                    <div className="flex items-center space-x-1">
                        <div className="w-3 h-0.5" style={{backgroundColor: 'rgba(31, 119, 180, 0.8)'}}></div>
                        <span>MA5</span>
                    </div>
                    <div className="flex items-center space-x-1">
                        <div className="w-3 h-0.5" style={{backgroundColor: 'rgba(255, 127, 14, 0.8)'}}></div>
                        <span>MA20</span>
                    </div>
                    <div className="flex items-center space-x-1">
                        <div className="w-3 h-0.5" style={{backgroundColor: 'rgba(44, 160, 44, 0.8)'}}></div>
                        <span>MA50</span>
                    </div>
                    {indicators?.bb_upper && (
                        <div className="flex items-center space-x-1">
                            <div className="w-3 h-0.5 border-dashed border-t-2" style={{borderColor: 'rgba(156, 39, 176, 0.6)'}}></div>
                            <span>布林带</span>
                        </div>
                    )}
                </div>
            </div>
            <div
                ref={chartContainerRef}
                className="border border-gray-200 rounded-lg"
                style={{height: `${height}px`}}
            />
        </div>
    );
}