/**
 * 錯誤頁面插圖集
 * 統一使用「Synapse Bot」角色，每種錯誤對應不同的處境與表情。
 * SVG 繪製語言：圓潤、輕鬆、幽默，配色柔和。
 */
import React from 'react';

/** ─────────────────────────────────────────────────────────
 *  404 — 小機器人飄進外太空，四處張望找不到目標頁面
 * ───────────────────────────────────────────────────────── */
export const Illus404: React.FC = () => (
  <svg width="220" height="185" viewBox="0 0 220 185" fill="none" xmlns="http://www.w3.org/2000/svg">
    {/* Ground shadow */}
    <ellipse cx="110" cy="178" rx="72" ry="12" fill="#dbeafe" opacity="0.5" />

    {/* Background planet */}
    <circle cx="184" cy="38" r="24" fill="#ede9fe" />
    <ellipse cx="184" cy="38" rx="33" ry="7" fill="none" stroke="#c4b5fd" strokeWidth="2" />

    {/* Stars */}
    <circle cx="22" cy="24" r="2" fill="#fbbf24" />
    <circle cx="48" cy="10" r="1.5" fill="white" />
    <circle cx="162" cy="14" r="2" fill="white" />
    <circle cx="14" cy="88" r="1.5" fill="#fbbf24" />
    <circle cx="198" cy="98" r="2" fill="white" />
    <circle cx="200" cy="148" r="1.5" fill="#fbbf24" opacity="0.7" />

    {/* Jetpack (behind body) */}
    <rect x="68" y="95" width="13" height="33" rx="5" fill="#9ca3af" />
    <rect x="70" y="99" width="9" height="8" rx="3" fill="#6b7280" />
    {/* Exhaust flame */}
    <ellipse cx="74" cy="131" rx="5" ry="3" fill="#fb923c" opacity="0.9" />
    <ellipse cx="74" cy="134" rx="3" ry="2" fill="#fde68a" />

    {/* Body */}
    <rect x="80" y="93" width="60" height="52" rx="12" fill="#3b82f6" />
    <rect x="89" y="103" width="42" height="24" rx="5" fill="#1d4ed8" />
    {/* Chest status lights */}
    <circle cx="100" cy="115" r="3.5" fill="#ef4444" />
    <circle cx="110" cy="115" r="3.5" fill="#facc15" />
    <circle cx="120" cy="115" r="3.5" fill="#22c55e" />

    {/* Head */}
    <rect x="82" y="48" width="56" height="48" rx="12" fill="#60a5fa" />
    {/* Visor */}
    <rect x="90" y="56" width="40" height="28" rx="7" fill="#1e3a8a" />

    {/* Left eye – looking upper-left (confused) */}
    <circle cx="102" cy="71" r="6.5" fill="white" />
    <circle cx="100" cy="69" r="3.5" fill="#1e3a8a" />
    <circle cx="99" cy="68" r="1.5" fill="white" />

    {/* Right eye – looking upper-right (confused) */}
    <circle cx="120" cy="71" r="6.5" fill="white" />
    <circle cx="122" cy="69" r="3.5" fill="#1e3a8a" />
    <circle cx="123" cy="68" r="1.5" fill="white" />

    {/* Confused squiggle mouth */}
    <path d="M 102 83 Q 106 80 110 83 Q 114 86 118 83" stroke="#1e3a8a" strokeWidth="2.5" strokeLinecap="round" fill="none" />

    {/* Antenna */}
    <line x1="110" y1="48" x2="110" y2="34" stroke="#60a5fa" strokeWidth="3" strokeLinecap="round" />
    <circle cx="110" cy="29" r="6" fill="#fbbf24" />
    <text x="106.5" y="33" fontSize="9" fill="#78350f" fontWeight="bold">?</text>

    {/* Left arm – raised in confusion */}
    <rect x="43" y="100" width="38" height="12" rx="6" fill="#3b82f6" transform="rotate(-22 62 106)" />
    <circle cx="43" cy="97" r="6.5" fill="#60a5fa" />

    {/* Right arm */}
    <rect x="139" y="95" width="38" height="12" rx="6" fill="#3b82f6" transform="rotate(22 158 101)" />
    <circle cx="179" cy="91" r="6.5" fill="#60a5fa" />

    {/* Legs */}
    <rect x="87" y="142" width="20" height="17" rx="5" fill="#3b82f6" />
    <rect x="113" y="142" width="20" height="17" rx="5" fill="#3b82f6" />
    <rect x="82" y="154" width="28" height="8" rx="4" fill="#1d4ed8" />
    <rect x="109" y="154" width="28" height="8" rx="4" fill="#1d4ed8" />

    {/* Floating ? marks */}
    <text x="24" y="70" fontSize="26" fill="#fbbf24" fontWeight="bold" opacity="0.9">?</text>
    <text x="172" y="68" fontSize="20" fill="#fbbf24" fontWeight="bold" opacity="0.7">?</text>
    <text x="8" y="135" fontSize="14" fill="#fbbf24" fontWeight="bold" opacity="0.4">?</text>
    <text x="152" y="142" fontSize="14" fill="#fbbf24" fontWeight="bold" opacity="0.45">?</text>
  </svg>
);

/** ─────────────────────────────────────────────────────────
 *  403 — 傲嬌大鎖擋住了門，表情嫌棄又帶點無奈
 * ───────────────────────────────────────────────────────── */
export const Illus403: React.FC = () => (
  <svg width="220" height="185" viewBox="0 0 220 185" fill="none" xmlns="http://www.w3.org/2000/svg">
    {/* Ground shadow */}
    <ellipse cx="110" cy="178" rx="68" ry="11" fill="#fef3c7" opacity="0.7" />

    {/* Door frame */}
    <rect x="26" y="28" width="17" height="148" rx="4" fill="#e5e7eb" />
    <rect x="177" y="28" width="17" height="148" rx="4" fill="#e5e7eb" />
    <rect x="24" y="22" width="172" height="15" rx="4" fill="#d1d5db" />
    {/* Door panel hint */}
    <rect x="43" y="43" width="134" height="130" rx="2" fill="#fffbeb" opacity="0.35" />

    {/* Shackle */}
    <path d="M 80 108 L 80 72 Q 80 48 110 48 Q 140 48 140 72 L 140 108"
      stroke="#d97706" strokeWidth="17" strokeLinecap="round" fill="none" />
    {/* Shackle inner highlight */}
    <path d="M 80 108 L 80 73 Q 80 56 110 56 Q 134 56 134 73 L 134 108"
      stroke="#fde68a" strokeWidth="5" strokeLinecap="round" fill="none" opacity="0.6" />

    {/* Lock body */}
    <rect x="62" y="106" width="96" height="74" rx="15" fill="#f59e0b" />
    {/* Top shine */}
    <rect x="72" y="114" width="36" height="13" rx="6" fill="#fde68a" opacity="0.45" />

    {/* Eyebrows – stern, angled inward */}
    <path d="M 78 126 Q 89 121 100 124" stroke="#78350f" strokeWidth="3.5" strokeLinecap="round" fill="none" />
    <path d="M 120 124 Q 131 121 142 126" stroke="#78350f" strokeWidth="3.5" strokeLinecap="round" fill="none" />

    {/* Eyes */}
    <circle cx="90" cy="135" r="8.5" fill="#78350f" />
    <circle cx="130" cy="135" r="8.5" fill="#78350f" />
    <circle cx="88" cy="133" r="3.5" fill="white" />
    <circle cx="128" cy="133" r="3.5" fill="white" />

    {/* Keyhole (acts as nose) */}
    <circle cx="110" cy="152" r="7" fill="#78350f" />
    <path d="M 107 158 L 107 168 L 113 168 L 113 158 Z" fill="#78350f" />

    {/* Badge "此路不通" */}
    <rect x="72" y="170" width="76" height="16" rx="8" fill="#d97706" />
    <text x="110" y="181" fontSize="10" fill="white" fontWeight="bold" textAnchor="middle" fontFamily="system-ui, -apple-system, sans-serif">此路不通</text>

    {/* Red ✕ on sides */}
    <line x1="33" y1="84" x2="45" y2="96" stroke="#ef4444" strokeWidth="3" strokeLinecap="round" opacity="0.55" />
    <line x1="45" y1="84" x2="33" y2="96" stroke="#ef4444" strokeWidth="3" strokeLinecap="round" opacity="0.55" />
    <line x1="175" y1="84" x2="187" y2="96" stroke="#ef4444" strokeWidth="3" strokeLinecap="round" opacity="0.55" />
    <line x1="187" y1="84" x2="175" y2="96" stroke="#ef4444" strokeWidth="3" strokeLinecap="round" opacity="0.55" />
  </svg>
);

/** ─────────────────────────────────────────────────────────
 *  500 — 伺服器中暑冒煙，表情驚恐，旁邊還在起火
 * ───────────────────────────────────────────────────────── */
export const Illus500: React.FC = () => (
  <svg width="220" height="185" viewBox="0 0 220 185" fill="none" xmlns="http://www.w3.org/2000/svg">
    {/* Ground shadow */}
    <ellipse cx="110" cy="178" rx="65" ry="11" fill="#fee2e2" opacity="0.7" />

    {/* Server body */}
    <rect x="54" y="64" width="112" height="104" rx="10" fill="#374151" />
    {/* Rack screw handles */}
    <rect x="58" y="72" width="15" height="5" rx="2.5" fill="#6b7280" />
    <rect x="147" y="72" width="15" height="5" rx="2.5" fill="#6b7280" />
    <rect x="58" y="155" width="15" height="5" rx="2.5" fill="#6b7280" />
    <rect x="147" y="155" width="15" height="5" rx="2.5" fill="#6b7280" />

    {/* Screen / face area */}
    <rect x="70" y="83" width="80" height="66" rx="7" fill="#1f2937" />

    {/* X eyes */}
    <line x1="81" y1="95" x2="95" y2="109" stroke="#ef4444" strokeWidth="4" strokeLinecap="round" />
    <line x1="95" y1="95" x2="81" y2="109" stroke="#ef4444" strokeWidth="4" strokeLinecap="round" />
    <line x1="106" y1="95" x2="120" y2="109" stroke="#ef4444" strokeWidth="4" strokeLinecap="round" />
    <line x1="120" y1="95" x2="106" y2="109" stroke="#ef4444" strokeWidth="4" strokeLinecap="round" />

    {/* Shocked open mouth */}
    <path d="M 86 125 Q 110 136 134 125" stroke="#ef4444" strokeWidth="3" strokeLinecap="round" fill="none" />

    {/* Sweat drop (panicking) */}
    <path d="M 138 94 Q 142 105 139 111 Q 136 105 138 94 Z" fill="#60a5fa" opacity="0.85" />

    {/* Status lights – all red / alarming */}
    <circle cx="86" cy="166" r="4" fill="#ef4444" />
    <circle cx="98" cy="166" r="4" fill="#ef4444" />
    <circle cx="110" cy="166" r="4" fill="#fbbf24" />
    <circle cx="122" cy="166" r="4" fill="#374151" />

    {/* Smoke wisps from top */}
    <path d="M 87 64 Q 82 48 87 34 Q 91 48 96 38 Q 97 52 92 62" fill="#d1d5db" opacity="0.55" />
    <path d="M 110 64 Q 105 46 110 32 Q 114 46 119 37 Q 120 51 115 62" fill="#d1d5db" opacity="0.45" />
    <path d="M 132 64 Q 128 48 132 35 Q 136 49 141 41 Q 141 55 136 62" fill="#d1d5db" opacity="0.38" />

    {/* Left flame */}
    <path d="M 62 168 Q 58 157 63 146 Q 61 154 67 150 Q 64 158 69 163 Q 67 156 73 152 Q 69 160 71 168 Z" fill="#fb923c" />
    <path d="M 64 168 Q 62 159 65 150 Q 63 157 68 154 Q 65 161 67 168 Z" fill="#fde68a" />

    {/* Right flame */}
    <path d="M 158 168 Q 162 157 157 146 Q 159 154 153 150 Q 156 158 151 163 Q 153 156 147 152 Q 151 160 149 168 Z" fill="#fb923c" />
    <path d="M 156 168 Q 158 159 155 150 Q 157 157 152 154 Q 155 161 153 168 Z" fill="#fde68a" />

    {/* Warning triangle */}
    <polygon points="150,52 163,30 176,52" fill="#f59e0b" stroke="white" strokeWidth="1.5" />
    <text x="163" y="49" fontSize="12" fill="white" fontWeight="bold" textAnchor="middle">!</text>
  </svg>
);

/** ─────────────────────────────────────────────────────────
 *  503 — 伺服器悠哉睡午覺，貼了「稍後回來」便利貼
 * ───────────────────────────────────────────────────────── */
export const Illus503: React.FC = () => (
  <svg width="220" height="185" viewBox="0 0 220 185" fill="none" xmlns="http://www.w3.org/2000/svg">
    {/* Ground shadow */}
    <ellipse cx="110" cy="178" rx="65" ry="11" fill="#dbeafe" opacity="0.45" />

    {/* Moon (crescent using mask) */}
    <defs>
      <mask id="moonMask">
        <circle cx="186" cy="33" r="21" fill="white" />
        <circle cx="198" cy="28" r="17" fill="black" />
      </mask>
    </defs>
    <rect x="165" y="12" width="42" height="42" fill="#fef9c3" mask="url(#moonMask)" />

    {/* Stars */}
    <circle cx="20" cy="22" r="2" fill="#fbbf24" opacity="0.8" />
    <circle cx="45" cy="12" r="1.5" fill="white" opacity="0.7" />
    <circle cx="165" cy="15" r="1.5" fill="white" opacity="0.8" />
    <circle cx="205" cy="52" r="2" fill="#fbbf24" opacity="0.6" />
    <circle cx="28" cy="55" r="1.5" fill="white" opacity="0.5" />

    {/* Server body */}
    <rect x="54" y="72" width="112" height="94" rx="12" fill="#3b82f6" />
    {/* Rack handles */}
    <rect x="58" y="80" width="15" height="5" rx="2.5" fill="#60a5fa" />
    <rect x="147" y="80" width="15" height="5" rx="2.5" fill="#60a5fa" />
    <rect x="58" y="153" width="15" height="5" rx="2.5" fill="#60a5fa" />
    <rect x="147" y="153" width="15" height="5" rx="2.5" fill="#60a5fa" />

    {/* Screen */}
    <rect x="70" y="90" width="80" height="60" rx="7" fill="#1d4ed8" />

    {/* Sleeping eyes (closed curved lines) */}
    <path d="M 83 118 Q 90 112 97 118" stroke="#93c5fd" strokeWidth="3.5" strokeLinecap="round" fill="none" />
    <path d="M 110 118 Q 117 112 124 118" stroke="#93c5fd" strokeWidth="3.5" strokeLinecap="round" fill="none" />

    {/* Peaceful smile */}
    <path d="M 93 132 Q 103 138 113 132" stroke="#93c5fd" strokeWidth="2.5" strokeLinecap="round" fill="none" />

    {/* Status lights – all off/dim */}
    <circle cx="90" cy="164" r="3.5" fill="#1d4ed8" />
    <circle cx="101" cy="164" r="3.5" fill="#1d4ed8" />
    <circle cx="112" cy="164" r="3.5" fill="#2563eb" />

    {/* ZZZ floating upward */}
    <text x="146" y="106" fontSize="14" fill="#93c5fd" fontWeight="bold" opacity="0.9">z</text>
    <text x="160" y="90" fontSize="18" fill="#93c5fd" fontWeight="bold" opacity="0.7">z</text>
    <text x="176" y="72" fontSize="22" fill="#bfdbfe" fontWeight="bold" opacity="0.5">z</text>

    {/* Post-it note (便利貼) leaning against server */}
    <rect x="24" y="86" width="28" height="30" rx="2" fill="#fef08a" transform="rotate(-8 38 101)" />
    {/* Note string */}
    <line x1="38" y1="86" x2="44" y2="76" stroke="#d1d5db" strokeWidth="1.5" strokeDasharray="2 2" transform="rotate(-8 38 101)" />
    <circle cx="46" cy="72" r="2.5" fill="#9ca3af" />
    {/* Note text */}
    <text x="38" y="100" fontSize="6.5" fill="#92400e" fontWeight="bold" textAnchor="middle" fontFamily="system-ui, -apple-system, sans-serif" transform="rotate(-8 38 101)">稍後</text>
    <text x="38" y="109" fontSize="6.5" fill="#92400e" fontWeight="bold" textAnchor="middle" fontFamily="system-ui, -apple-system, sans-serif" transform="rotate(-8 38 101)">回來</text>
  </svg>
);

/** ─────────────────────────────────────────────────────────
 *  Network — 訊號完全消失，網路線被拔掉，信號條全空
 * ───────────────────────────────────────────────────────── */
export const IllusNetwork: React.FC = () => (
  <svg width="220" height="185" viewBox="0 0 220 185" fill="none" xmlns="http://www.w3.org/2000/svg">
    {/* Ground shadow */}
    <ellipse cx="110" cy="178" rx="60" ry="11" fill="#f3f4f6" opacity="0.9" />

    {/* WiFi arcs – fading from inside out */}

    {/* Outer arc – very faint, barely there */}
    <path d="M 30 115 Q 110 18 190 115" stroke="#e5e7eb" strokeWidth="8" strokeLinecap="round" fill="none" />

    {/* Middle arc – dashed, weak signal */}
    <path d="M 55 115 Q 110 42 165 115" stroke="#d1d5db" strokeWidth="9" strokeLinecap="round" fill="none" strokeDasharray="18 14" />

    {/* Inner arc – LEFT half solid, RIGHT half broken */}
    <path d="M 80 115 Q 95 74 110 70" stroke="#9ca3af" strokeWidth="10" strokeLinecap="round" fill="none" />
    <path d="M 110 70 Q 125 74 140 115" stroke="#9ca3af" strokeWidth="10" strokeLinecap="round" fill="none" opacity="0.22" />

    {/* Crack / break at the top of the inner arc */}
    <path d="M 106 73 L 110 80 L 114 73" stroke="#ef4444" strokeWidth="2.5" fill="none" strokeLinecap="round" strokeLinejoin="round" />

    {/* Center dot with sad face */}
    <circle cx="110" cy="135" r="13" fill="#6b7280" />
    <circle cx="105" cy="131" r="2.5" fill="white" />
    <circle cx="115" cy="131" r="2.5" fill="white" />
    {/* Sad mouth */}
    <path d="M 104 142 Q 110 139 116 142" stroke="white" strokeWidth="2" strokeLinecap="round" fill="none" />

    {/* Disconnected cable on lower left */}
    <path d="M 36 148 Q 44 162 56 164 L 70 164" stroke="#9ca3af" strokeWidth="4" strokeLinecap="round" fill="none" />
    <rect x="68" y="159" width="18" height="10" rx="3" fill="#6b7280" />
    {/* Plug prongs */}
    <line x1="74" y1="169" x2="74" y2="176" stroke="#4b5563" strokeWidth="2.5" strokeLinecap="round" />
    <line x1="80" y1="169" x2="80" y2="176" stroke="#4b5563" strokeWidth="2.5" strokeLinecap="round" />

    {/* Spark at break point */}
    <path d="M 34 146 L 29 157 L 38 154 L 33 165" stroke="#fbbf24" strokeWidth="2.5" fill="none" strokeLinecap="round" strokeLinejoin="round" />

    {/* Signal bars (all empty) on right */}
    <rect x="156" y="158" width="7" height="13" rx="2" fill="none" stroke="#d1d5db" strokeWidth="1.5" />
    <rect x="167" y="149" width="7" height="22" rx="2" fill="none" stroke="#d1d5db" strokeWidth="1.5" />
    <rect x="178" y="139" width="7" height="32" rx="2" fill="none" stroke="#d1d5db" strokeWidth="1.5" />
    <rect x="189" y="128" width="7" height="43" rx="2" fill="none" stroke="#d1d5db" strokeWidth="1.5" />
    {/* Only first bar filled, others empty */}
    <rect x="156" y="158" width="7" height="13" rx="2" fill="#d1d5db" />
  </svg>
);
