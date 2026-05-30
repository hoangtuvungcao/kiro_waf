/**
 * Kiro WAF — Chart Engine
 * Provides chart rendering functions for the admin dashboard.
 * Uses Chart.js (bundled locally) and reads brand colors from CSS custom properties.
 *
 * Dependencies: chart.min.js must be loaded before this file.
 */

/* global Chart */

(function (global) {
  'use strict';

  /**
   * Read a CSS custom property value from the document root.
   * @param {string} name - CSS variable name (e.g., '--kiro-primary')
   * @returns {string} The computed value or fallback
   */
  function getCSSVar(name, fallback) {
    if (typeof getComputedStyle === 'undefined' || !document.documentElement) {
      return fallback || '';
    }
    var value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback || '';
  }

  /**
   * Get brand colors from CSS custom properties for chart segments.
   * @returns {object} Color map for license statuses
   */
  function getBrandColors() {
    return {
      active: getCSSVar('--kiro-success', '#10b981'),
      suspended: getCSSVar('--kiro-warning', '#f59e0b'),
      revoked: getCSSVar('--kiro-danger', '#ef4444'),
      expired: getCSSVar('--kiro-text-secondary', '#a0a0b0')
    };
  }

  /**
   * Get common chart options for dark theme styling.
   * @returns {object} Chart.js options for dark theme
   */
  function getDarkThemeDefaults() {
    var textColor = getCSSVar('--kiro-text-primary', '#f0f0f0');
    var textSecondary = getCSSVar('--kiro-text-secondary', '#a0a0b0');
    var borderColor = getCSSVar('--kiro-border', '#2a2a3e');

    return {
      responsive: true,
      maintainAspectRatio: true,
      plugins: {
        legend: {
          labels: {
            color: textColor,
            font: { size: 12 }
          }
        },
        tooltip: {
          enabled: true,
          backgroundColor: 'rgba(26, 26, 46, 0.95)',
          titleColor: textColor,
          bodyColor: textSecondary,
          borderColor: borderColor,
          borderWidth: 1,
          cornerRadius: 6,
          padding: 10
        }
      }
    };
  }

  /**
   * Show a placeholder message when no data is available.
   * @param {HTMLElement} container - The container element
   * @param {string} message - The message to display
   */
  function showEmptyState(container, message) {
    var placeholder = document.createElement('div');
    placeholder.className = 'kiro-chart-empty';
    placeholder.style.display = 'flex';
    placeholder.style.alignItems = 'center';
    placeholder.style.justifyContent = 'center';
    placeholder.style.minHeight = '200px';
    placeholder.style.color = getCSSVar('--kiro-text-secondary', '#a0a0b0');
    placeholder.style.fontSize = '14px';
    placeholder.style.fontStyle = 'italic';
    placeholder.textContent = message || 'No data available';
    container.innerHTML = '';
    container.appendChild(placeholder);
  }

  /**
   * Create a canvas element inside the container for Chart.js rendering.
   * @param {HTMLElement} container - The container element
   * @returns {CanvasRenderingContext2D} The 2D context of the canvas
   */
  function createCanvas(container) {
    container.innerHTML = '';
    var canvas = document.createElement('canvas');
    canvas.style.width = '100%';
    canvas.style.maxHeight = '400px';
    container.appendChild(canvas);
    return canvas.getContext('2d');
  }

  /**
   * Render a license distribution doughnut/pie chart.
   * Displays active, suspended, revoked, and expired license counts.
   *
   * @param {HTMLElement} container - DOM element to render the chart into
   * @param {object} data - License statistics
   * @param {number} data.active - Count of active licenses
   * @param {number} data.suspended - Count of suspended licenses
   * @param {number} data.revoked - Count of revoked licenses
   * @param {number} data.expired - Count of expired licenses
   * @returns {Chart|null} The Chart.js instance or null if no data
   */
  function renderLicenseDistribution(container, data) {
    if (!container) {
      console.error('[kiro-charts] renderLicenseDistribution: container is required');
      return null;
    }

    if (!data || typeof data !== 'object') {
      showEmptyState(container, 'No license data available');
      return null;
    }

    var active = data.active || 0;
    var suspended = data.suspended || 0;
    var revoked = data.revoked || 0;
    var expired = data.expired || 0;
    var total = active + suspended + revoked + expired;

    if (total === 0) {
      showEmptyState(container, 'No license data available');
      return null;
    }

    var colors = getBrandColors();
    var ctx = createCanvas(container);
    var defaults = getDarkThemeDefaults();

    var chart = new Chart(ctx, {
      type: 'doughnut',
      data: {
        labels: ['Active', 'Suspended', 'Revoked', 'Expired'],
        datasets: [{
          data: [active, suspended, revoked, expired],
          backgroundColor: [
            colors.active,
            colors.suspended,
            colors.revoked,
            colors.expired
          ],
          borderColor: getCSSVar('--kiro-surface', '#1a1a2e'),
          borderWidth: 2,
          hoverBorderColor: getCSSVar('--kiro-text-primary', '#f0f0f0'),
          hoverBorderWidth: 2
        }]
      },
      options: {
        responsive: defaults.responsive,
        maintainAspectRatio: defaults.maintainAspectRatio,
        cutout: '55%',
        plugins: {
          legend: {
            position: 'bottom',
            labels: {
              color: defaults.plugins.legend.labels.color,
              font: defaults.plugins.legend.labels.font,
              padding: 16,
              usePointStyle: true,
              pointStyle: 'circle'
            }
          },
          tooltip: {
            enabled: defaults.plugins.tooltip.enabled,
            backgroundColor: defaults.plugins.tooltip.backgroundColor,
            titleColor: defaults.plugins.tooltip.titleColor,
            bodyColor: defaults.plugins.tooltip.bodyColor,
            borderColor: defaults.plugins.tooltip.borderColor,
            borderWidth: defaults.plugins.tooltip.borderWidth,
            cornerRadius: defaults.plugins.tooltip.cornerRadius,
            padding: defaults.plugins.tooltip.padding,
            callbacks: {
              label: function (context) {
                var label = context.label || '';
                var value = context.parsed || 0;
                var percentage = total > 0 ? ((value / total) * 100).toFixed(1) : 0;
                return label + ': ' + value + ' (' + percentage + '%)';
              }
            }
          }
        }
      }
    });

    return chart;
  }

  /**
   * Format an ISO timestamp to a short hour label (e.g., "14:00").
   * @param {string} isoStr - ISO 8601 timestamp
   * @returns {string} Formatted hour label
   */
  function formatHourLabel(isoStr) {
    try {
      var d = new Date(isoStr);
      if (isNaN(d.getTime())) return isoStr;
      var hours = d.getHours().toString().padStart(2, '0');
      var minutes = d.getMinutes().toString().padStart(2, '0');
      return hours + ':' + minutes;
    } catch (e) {
      return isoStr;
    }
  }

  /**
   * Format an ISO timestamp to a short date label (e.g., "Jan 15").
   * @param {string} isoStr - ISO 8601 timestamp
   * @returns {string} Formatted date label
   */
  function formatDateLabel(isoStr) {
    try {
      var d = new Date(isoStr);
      if (isNaN(d.getTime())) return isoStr;
      var months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
                    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
      return months[d.getMonth()] + ' ' + d.getDate();
    } catch (e) {
      return isoStr;
    }
  }

  /**
   * Render a heartbeat timeline line chart showing hourly heartbeat counts for 24h.
   * Displays the number of heartbeats received per 1-hour interval over the past 24 hours.
   *
   * @param {HTMLElement} container - DOM element to render the chart into
   * @param {Array} data - Array of hourly heartbeat data
   * @param {string} data[].hour - ISO 8601 timestamp for the hour bucket (e.g., "2024-01-15T14:00:00Z")
   * @param {number} data[].count - Number of heartbeats received in that hour
   * @returns {Chart|null} The Chart.js instance or null if no data
   */
  function renderHeartbeatTimeline(container, data) {
    if (!container) {
      console.error('[kiro-charts] renderHeartbeatTimeline: container is required');
      return null;
    }

    if (!data || !Array.isArray(data) || data.length === 0) {
      showEmptyState(container, 'No heartbeat data available');
      return null;
    }

    // Filter out invalid entries
    var validData = data.filter(function (item) {
      return item && typeof item.hour === 'string' && typeof item.count === 'number';
    });

    if (validData.length === 0) {
      showEmptyState(container, 'No heartbeat data available');
      return null;
    }

    var labels = validData.map(function (item) { return formatHourLabel(item.hour); });
    var counts = validData.map(function (item) { return item.count; });

    var primaryColor = getCSSVar('--kiro-primary', '#0d9488');
    var accentColor = getCSSVar('--kiro-accent', '#14b8a6');
    var ctx = createCanvas(container);
    var defaults = getDarkThemeDefaults();
    var textColor = getCSSVar('--kiro-text-primary', '#f0f0f0');
    var textSecondary = getCSSVar('--kiro-text-secondary', '#a0a0b0');
    var borderColor = getCSSVar('--kiro-border', '#2a2a3e');

    var chart = new Chart(ctx, {
      type: 'line',
      data: {
        labels: labels,
        datasets: [{
          label: 'Heartbeats',
          data: counts,
          borderColor: primaryColor,
          backgroundColor: 'rgba(13, 148, 136, 0.15)',
          pointBackgroundColor: accentColor,
          pointBorderColor: primaryColor,
          pointHoverBackgroundColor: textColor,
          pointHoverBorderColor: primaryColor,
          pointRadius: 3,
          pointHoverRadius: 6,
          borderWidth: 2,
          fill: true,
          tension: 0.3
        }]
      },
      options: {
        responsive: defaults.responsive,
        maintainAspectRatio: defaults.maintainAspectRatio,
        interaction: {
          mode: 'index',
          intersect: false
        },
        scales: {
          x: {
            grid: {
              color: borderColor,
              drawBorder: false
            },
            ticks: {
              color: textSecondary,
              font: { size: 11 },
              maxRotation: 45,
              autoSkip: true,
              maxTicksLimit: 12
            }
          },
          y: {
            beginAtZero: true,
            grid: {
              color: borderColor,
              drawBorder: false
            },
            ticks: {
              color: textSecondary,
              font: { size: 11 },
              precision: 0
            },
            title: {
              display: true,
              text: 'Heartbeat Count',
              color: textSecondary,
              font: { size: 12 }
            }
          }
        },
        plugins: {
          legend: {
            display: false
          },
          tooltip: {
            enabled: true,
            backgroundColor: defaults.plugins.tooltip.backgroundColor,
            titleColor: defaults.plugins.tooltip.titleColor,
            bodyColor: defaults.plugins.tooltip.bodyColor,
            borderColor: defaults.plugins.tooltip.borderColor,
            borderWidth: defaults.plugins.tooltip.borderWidth,
            cornerRadius: defaults.plugins.tooltip.cornerRadius,
            padding: defaults.plugins.tooltip.padding,
            callbacks: {
              title: function (tooltipItems) {
                var idx = tooltipItems[0].dataIndex;
                // Show full hour from original data
                return 'Hour: ' + validData[idx].hour;
              },
              label: function (context) {
                return 'Heartbeats: ' + context.parsed.y;
              }
            }
          }
        }
      }
    });

    return chart;
  }

  /**
   * Render a release history scatter/line chart with version vs. creation date.
   * Plots release versions on the Y-axis against their creation dates on the X-axis.
   *
   * @param {HTMLElement} container - DOM element to render the chart into
   * @param {Array} data - Array of release data
   * @param {string} data[].version - Semantic version string (e.g., "1.0.0")
   * @param {string} data[].created_at - ISO 8601 timestamp of release creation
   * @returns {Chart|null} The Chart.js instance or null if no data
   */
  function renderReleaseHistory(container, data) {
    if (!container) {
      console.error('[kiro-charts] renderReleaseHistory: container is required');
      return null;
    }

    if (!data || !Array.isArray(data) || data.length === 0) {
      showEmptyState(container, 'No release data available');
      return null;
    }

    // Filter out invalid entries
    var validData = data.filter(function (item) {
      return item && typeof item.version === 'string' && typeof item.created_at === 'string';
    });

    if (validData.length === 0) {
      showEmptyState(container, 'No release data available');
      return null;
    }

    // Sort by creation date ascending
    validData.sort(function (a, b) {
      return new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
    });

    var labels = validData.map(function (item) { return formatDateLabel(item.created_at); });
    var versions = validData.map(function (item) { return item.version; });
    // Use index as Y value for plotting (versions are categorical)
    var yValues = validData.map(function (_item, idx) { return idx; });

    var primaryColor = getCSSVar('--kiro-primary', '#0d9488');
    var accentColor = getCSSVar('--kiro-accent', '#14b8a6');
    var ctx = createCanvas(container);
    var defaults = getDarkThemeDefaults();
    var textColor = getCSSVar('--kiro-text-primary', '#f0f0f0');
    var textSecondary = getCSSVar('--kiro-text-secondary', '#a0a0b0');
    var borderColor = getCSSVar('--kiro-border', '#2a2a3e');

    var chart = new Chart(ctx, {
      type: 'line',
      data: {
        labels: labels,
        datasets: [{
          label: 'Releases',
          data: yValues,
          borderColor: accentColor,
          backgroundColor: 'rgba(20, 184, 166, 0.15)',
          pointBackgroundColor: primaryColor,
          pointBorderColor: accentColor,
          pointHoverBackgroundColor: textColor,
          pointHoverBorderColor: accentColor,
          pointRadius: 5,
          pointHoverRadius: 8,
          pointStyle: 'rectRot',
          borderWidth: 2,
          fill: false,
          tension: 0,
          showLine: true
        }]
      },
      options: {
        responsive: defaults.responsive,
        maintainAspectRatio: defaults.maintainAspectRatio,
        interaction: {
          mode: 'index',
          intersect: false
        },
        scales: {
          x: {
            grid: {
              color: borderColor,
              drawBorder: false
            },
            ticks: {
              color: textSecondary,
              font: { size: 11 },
              maxRotation: 45,
              autoSkip: true
            },
            title: {
              display: true,
              text: 'Release Date',
              color: textSecondary,
              font: { size: 12 }
            }
          },
          y: {
            grid: {
              color: borderColor,
              drawBorder: false
            },
            ticks: {
              color: textSecondary,
              font: { size: 11 },
              callback: function (value) {
                // Show version string instead of numeric index
                var idx = Math.round(value);
                if (idx >= 0 && idx < versions.length) {
                  return versions[idx];
                }
                return '';
              },
              stepSize: 1
            },
            title: {
              display: true,
              text: 'Version',
              color: textSecondary,
              font: { size: 12 }
            },
            min: -0.5,
            max: versions.length - 0.5
          }
        },
        plugins: {
          legend: {
            display: false
          },
          tooltip: {
            enabled: true,
            backgroundColor: defaults.plugins.tooltip.backgroundColor,
            titleColor: defaults.plugins.tooltip.titleColor,
            bodyColor: defaults.plugins.tooltip.bodyColor,
            borderColor: defaults.plugins.tooltip.borderColor,
            borderWidth: defaults.plugins.tooltip.borderWidth,
            cornerRadius: defaults.plugins.tooltip.cornerRadius,
            padding: defaults.plugins.tooltip.padding,
            callbacks: {
              title: function (tooltipItems) {
                var idx = tooltipItems[0].dataIndex;
                return 'Version: ' + validData[idx].version;
              },
              label: function (context) {
                var idx = context.dataIndex;
                return 'Released: ' + validData[idx].created_at;
              }
            }
          }
        }
      }
    });

    return chart;
  }

  // Export the chart engine to the global scope
  var KiroCharts = {
    renderLicenseDistribution: renderLicenseDistribution,
    renderHeartbeatTimeline: renderHeartbeatTimeline,
    renderReleaseHistory: renderReleaseHistory,
    // Utility functions exposed for testing and future use
    _getBrandColors: getBrandColors,
    _showEmptyState: showEmptyState,
    _getDarkThemeDefaults: getDarkThemeDefaults
  };

  // Support both module and global usage
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = KiroCharts;
  } else {
    global.KiroCharts = KiroCharts;
  }

})(typeof window !== 'undefined' ? window : this);
