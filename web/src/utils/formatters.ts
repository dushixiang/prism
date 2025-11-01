import {PRICE_UNIT} from '@/constants/currency';

const numberFormatters = new Map<number, Intl.NumberFormat>();

const getNumberFormatter = (fractionDigits: number) => {
    if (!numberFormatters.has(fractionDigits)) {
        numberFormatters.set(fractionDigits, new Intl.NumberFormat('zh-CN', {
            minimumFractionDigits: fractionDigits,
            maximumFractionDigits: fractionDigits,
        }));
    }
    return numberFormatters.get(fractionDigits)!;
};

const formatNumericValue = (value: number | undefined, fractionDigits: number) => {
    if (value === undefined || Number.isNaN(value)) {
        return '-';
    }
    return getNumberFormatter(fractionDigits).format(value);
};

const formatWithUnit = (value: number | undefined, fractionDigits: number) => {
    const formattedValue = formatNumericValue(value, fractionDigits);
    return formattedValue === '-' ? formattedValue : `${formattedValue} ${PRICE_UNIT}`;
};

export const formatCurrency = (value: number | undefined, fractionDigits = 2) => {
    return formatWithUnit(value, fractionDigits);
};

export const formatPercent = (value: number | undefined) => {
    if (value === undefined || Number.isNaN(value)) {
        return '-';
    }
    const sign = value > 0 ? '+' : '';
    return `${sign}${value.toFixed(2)}%`;
};

export const formatNumber = (value: number | undefined, fractionDigits = 2) => {
    return formatNumericValue(value, fractionDigits);
};

export const formatPrice = (value: number | undefined, fractionDigits = 4) => {
    return formatWithUnit(value, fractionDigits);
};

export const formatDateTime = (value?: string) => {
    if (!value) {
        return '-';
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return value;
    }
    return date.toLocaleString('zh-CN', {
        timeZone: 'Asia/Shanghai',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
    });
};

export const formatTimestampToCST = (epochSeconds: number) => {
    if (!Number.isFinite(epochSeconds)) {
        return '-';
    }
    return new Date(epochSeconds * 1000).toLocaleString('zh-CN', {
        timeZone: 'Asia/Shanghai',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
    });
};

export const getErrorMessage = (error: unknown) => {
    if (!error) {
        return '';
    }
    return error instanceof Error ? error.message : '未知错误';
};

export const getPnlColor = (value: number | undefined) => {
    if (value === undefined || Number.isNaN(value)) {
        return 'text-slate-600';
    }
    if (value > 0) {
        return 'text-emerald-600';
    }
    if (value < 0) {
        return 'text-rose-600';
    }
    return 'text-slate-600';
};
