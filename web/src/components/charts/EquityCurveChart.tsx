import {useEffect, useRef} from 'react';
import {
    type BarData,
    type BusinessDay,
    ColorType,
    createChart,
    type IChartApi,
    type ISeriesApi,
    type LineData,
    LineSeries,
    type MouseEventParams,
    type Time,
    type UTCTimestamp,
} from 'lightweight-charts';
import {formatCurrency, formatTimestampToCST} from '@/utils/formatters.ts';
import type {EquityCurveDataPoint} from '@/types/trading.ts';

interface EquityCurveChartProps {
    data: EquityCurveDataPoint[];
    initialBalance: number;
}

const toEpochSeconds = (time: Time): number | null => {
    if (typeof time === 'number') {
        return time;
    }
    if (typeof time === 'string') {
        const parsed = Number(time);
        return Number.isFinite(parsed) ? parsed : null;
    }
    const businessDay = time as BusinessDay;
    if (typeof businessDay?.year === 'number' && typeof businessDay?.month === 'number' && typeof businessDay?.day === 'number') {
        return Math.floor(Date.UTC(businessDay.year, businessDay.month - 1, businessDay.day) / 1000);
    }
    return null;
};

export const EquityCurveChart = ({data, initialBalance}: EquityCurveChartProps) => {
    const chartContainerRef = useRef<HTMLDivElement>(null);
    const chartRef = useRef<IChartApi | null>(null);
    const seriesRef = useRef<ISeriesApi<any> | null>(null);

    useEffect(() => {
        if (!chartContainerRef.current || data.length === 0) return;

        chartContainerRef.current.style.position = 'relative';

        // 创建图表
        const chart = createChart(chartContainerRef.current, {
            layout: {
                background: {type: ColorType.Solid, color: '#ffffff'},
                textColor: '#64748b',
            },
            width: chartContainerRef.current.clientWidth,
            height: chartContainerRef.current.clientHeight,
            grid: {
                vertLines: {color: '#f1f5f9', style: 1},
                horzLines: {color: '#f1f5f9', style: 1},
            },
            rightPriceScale: {
                borderColor: '#e2e8f0',
                textColor: '#64748b',
            },
            timeScale: {
                borderColor: '#e2e8f0',
                timeVisible: true,
                secondsVisible: false,
            },
            crosshair: {
                mode: 1,
                vertLine: {
                    color: '#cbd5e1',
                    width: 1,
                    style: 3,
                    labelBackgroundColor: '#2862E3',
                },
                horzLine: {
                    color: '#cbd5e1',
                    width: 1,
                    style: 3,
                    labelBackgroundColor: '#2862E3',
                },
            },
        });

        chartRef.current = chart;

        // 创建线系列
        const lineSeries = chart.addSeries(LineSeries, {
            color: '#2862E3',
            lineWidth: 3,
            priceFormat: {
                type: 'price',
                precision: 2,
                minMove: 0.01,
            },
            lastValueVisible: true,
            priceLineVisible: true,
        });

        seriesRef.current = lineSeries;

        // 转换数据格式，并按时间排序、去重
        console.log('Original data points:', data.length);

        // 转换为数组并按时间升序排序
        const chartData = data
            .map((item) => ({
                time: item.timestamp as UTCTimestamp,
                value: item.total_balance,
            }));

        if (chartData.length === 0) {
            console.error('No valid chart data after filtering');
            return;
        }

        lineSeries.setData(chartData);

        const tooltip = document.createElement('div');
        tooltip.style.position = 'absolute';
        tooltip.style.display = 'none';
        tooltip.style.pointerEvents = 'none';
        tooltip.style.zIndex = '50';
        tooltip.style.padding = '6px 10px';
        tooltip.style.borderRadius = '8px';
        tooltip.style.background = 'rgba(30, 41, 59, 0.92)';
        tooltip.style.color = '#f8fafc';
        tooltip.style.fontSize = '12px';
        tooltip.style.lineHeight = '1.4';
        tooltip.style.border = '1px solid rgba(148, 163, 184, 0.35)';
        tooltip.style.boxShadow = '0 12px 32px rgba(15, 23, 42, 0.25)';
        tooltip.style.whiteSpace = 'nowrap';

        chartContainerRef.current.appendChild(tooltip);

        const handleCrosshairMove = (param: MouseEventParams<Time>) => {
            const container = chartContainerRef.current;
            const point = param.point;
            const timeValue = param.time;

            if (!container || !point || timeValue === undefined) {
                tooltip.style.display = 'none';
                return;
            }

            const x = Number(point.x);
            const y = Number(point.y);
            if (!Number.isFinite(x) || !Number.isFinite(y) || x < 0 || y < 0 ||
                x > container.clientWidth || y > container.clientHeight) {
                tooltip.style.display = 'none';
                return;
            }

            const seriesValue = param.seriesData.get(lineSeries);
            let price: number | undefined;
            if (seriesValue) {
                const typedSeries = seriesValue as Partial<LineData<Time>> & Partial<BarData<Time>>;
                if (typeof typedSeries.value === 'number') {
                    price = typedSeries.value;
                } else if (typeof typedSeries.close === 'number') {
                    price = typedSeries.close;
                }
            }

            if (price === undefined) {
                tooltip.style.display = 'none';
                return;
            }

            const epochSeconds = toEpochSeconds(timeValue as Time);
            if (!epochSeconds) {
                tooltip.style.display = 'none';
                return;
            }

            tooltip.innerHTML = `
                <div style="font-size:11px;color:#cbd5f5;margin-bottom:2px;">${formatTimestampToCST(epochSeconds)}</div>
                <div style="font-size:12px;color:#f8fafc;font-weight:600;">余额：${formatCurrency(price)}</div>
            `;

            tooltip.style.display = 'block';
            const tooltipRect = tooltip.getBoundingClientRect();
            const containerWidth = container.clientWidth;
            const containerHeight = container.clientHeight;

            let left = x;
            let top = y - 12;

            if (left < tooltipRect.width / 2 + 8) {
                left = tooltipRect.width / 2 + 8;
            } else if (left > containerWidth - tooltipRect.width / 2 - 8) {
                left = containerWidth - tooltipRect.width / 2 - 8;
            }

            if (top < tooltipRect.height + 12) {
                top = y + tooltipRect.height + 12;
            }
            if (top > containerHeight - 12) {
                top = containerHeight - 12;
            }

            tooltip.style.left = `${left}px`;
            tooltip.style.top = `${top}px`;
        };

        chart.subscribeCrosshairMove(handleCrosshairMove);

        // 添加初始余额参考线（需要至少2个不同时间点）
        if (initialBalance > 0 && chartData.length >= 2) {
            const minTime = chartData[0].time as number;
            const maxTime = chartData[chartData.length - 1].time as number;

            // 只有当最小时间和最大时间不同时才添加参考线
            if (minTime !== maxTime) {
                const referenceLine = chart.addSeries(LineSeries, {
                    color: '#94a3b8',
                    lineWidth: 1,
                    lineStyle: 3,
                    priceFormat: {
                        type: 'price',
                        precision: 2,
                        minMove: 0.01,
                    },
                    lastValueVisible: false,
                    priceLineVisible: false,
                });

                referenceLine.setData([
                    {time: minTime as Time, value: initialBalance},
                    {time: maxTime as Time, value: initialBalance},
                ]);
            }
        }

        // 自适应内容
        chart.timeScale().fitContent();

        // 响应式调整
        const handleResize = () => {
            if (chartContainerRef.current) {
                chart.applyOptions({
                    width: chartContainerRef.current.clientWidth,
                    height: chartContainerRef.current.clientHeight,
                });
            }
        };

        window.addEventListener('resize', handleResize);

        return () => {
            window.removeEventListener('resize', handleResize);
            chart.unsubscribeCrosshairMove(handleCrosshairMove);
            if (tooltip.parentNode) {
                tooltip.parentNode.removeChild(tooltip);
            }
            chart.remove();
        };
    }, [data, initialBalance]);

    if (data.length === 0) {
        return (
            <div className="flex h-full items-center justify-center text-slate-400">
                暂无资金曲线数据
            </div>
        );
    }

    return <div ref={chartContainerRef} className="relative h-full w-full"/>;
};
