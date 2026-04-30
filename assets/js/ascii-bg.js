// ascii-bg.js
// A premium, static grid of flickering ASCII characters to represent the cognitive map-reduce engine.

(function() {
  const canvas = document.getElementById('hero-ascii-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');

  let width, height;
  let cols, rows;
  const fontSize = 16;
  const chars = 'アカサタナハマヤラワガザダバパキシチニヒミリギジヂビピウクスツヌフムユルグズヅブプエケセテネヘメレゲゼデベペオコソトノホモヨロヲゴン1234567890OPENLOBSTER';
  
  // Create the grid state
  let grid = [];

  function init() {
    width = canvas.width = window.innerWidth;
    // We only need the hero height, but innerHeight is safe for the absolute canvas
    height = canvas.height = document.querySelector('.hero').offsetHeight;

    cols = Math.floor(width / fontSize) + 1;
    rows = Math.floor(height / fontSize) + 1;

    grid = [];
    for (let i = 0; i < cols; i++) {
      grid[i] = [];
      for (let j = 0; j < rows; j++) {
        // Only populate a subset to keep it minimal and premium (like a sparse neural network)
        if (Math.random() > 0.70) {
          grid[i][j] = {
            char: chars.charAt(Math.floor(Math.random() * chars.length)),
            opacity: Math.random() * 0.8,
            targetOpacity: Math.random() * 0.8,
            speed: 0.02 + Math.random() * 0.04,
            active: true
          };
        } else {
          grid[i][j] = { active: false };
        }
      }
    }
  }

  function draw() {
    ctx.clearRect(0, 0, width, height);
    ctx.font = fontSize + "px 'JetBrains Mono', 'Fira Code', monospace";
    ctx.textAlign = 'center';

    // Get the CSS accent color
    const rootStyles = getComputedStyle(document.documentElement);
    const accentRgb = rootStyles.getPropertyValue('--accent-rgb').trim() || '255, 255, 255';

    for (let i = 0; i < cols; i++) {
      for (let j = 0; j < rows; j++) {
        let cell = grid[i][j];
        if (!cell.active) continue;

        // Smooth opacity transition
        cell.opacity += (cell.targetOpacity - cell.opacity) * cell.speed;
        
        // Randomly pick new target opacity or swap character to simulate "thinking"
        if (Math.random() > 0.98) {
          cell.targetOpacity = Math.random() * 0.8;
          if (Math.random() > 0.4) {
            cell.char = chars.charAt(Math.floor(Math.random() * chars.length));
          }
        }

        ctx.fillStyle = `rgba(${accentRgb}, ${cell.opacity})`;
        // Add a slight random offset to make it look like a map-reduce clustering
        ctx.fillText(cell.char, i * fontSize + (fontSize/2), j * fontSize + fontSize);
      }
    }

    requestAnimationFrame(draw);
  }

  window.addEventListener('resize', init);
  init();
  draw();
})();
