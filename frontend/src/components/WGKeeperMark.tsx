type Props = {
  className?: string;
};

// The "wg" of the wordmark on its own, for places too narrow for the full name.
export const WGKeeperMark = ({ className }: Props) => (
  <svg
    viewBox="0 0 113 73.8"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    role="img"
    aria-label="wgkeeper"
  >
    <path
      d="M14.7 58.8 2 15.9H14.1L21.4 45L29.4 15.9H39.1L47 45L54.5 15.9H66.6L53.8 58.8H42.2L34.2 32L26.3 58.8ZM89.9 71.8Q80.7 71.8 75.9 68.4Q71.1 65 69.7 59.8L82 58.9Q82.6 61 84.4 62.2Q86.2 63.4 89.9 63.4Q94.2 63.4 96.5 61.3Q98.8 59.3 98.8 54.8V50.2Q97.3 53.2 93.9 55Q90.6 56.8 86.4 56.8Q81.1 56.8 77 54.2Q73 51.7 70.7 47Q68.4 42.2 68.4 35.9Q68.4 29.8 70.7 25Q73 20.3 77 17.6Q81 15 86.2 15Q91 15 94.2 17Q97.4 19.1 99.1 22.6V15.9H110.9V54.6Q110.9 60.3 108.2 64.2Q105.6 68 100.9 69.9Q96.2 71.8 89.9 71.8ZM89.8 47.9Q93.9 47.9 96.4 44.8Q98.9 41.6 98.9 35.8Q99 30.2 96.5 27Q94 23.8 89.8 23.8Q85.5 23.8 83.1 27Q80.7 30.3 80.7 35.8Q80.7 41.4 83.2 44.6Q85.7 47.9 89.8 47.9Z"
      className="fill-brand"
    />
  </svg>
);
