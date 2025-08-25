interface LogoProps {
    size?: number;
    className?: string;
}

export function Logo({size = 24, className = ""}: LogoProps) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 24 24"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            className={className}
        >
            <defs>
                <linearGradient id="logoGradient" x1="0%" y1="0%" x2="100%" y2="100%">
                    <stop offset="0%" stopColor="#3b82f6"/>
                    <stop offset="100%" stopColor="#1d4ed8"/>
                </linearGradient>
            </defs>
            <path
                d="M12 2L22 20H2L12 2Z"
                fill="url(#logoGradient)"
                stroke="none"
            />
        </svg>
    );
}
