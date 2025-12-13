/**
 * Utility functions for the web frontend
 *
 * This module provides helper functions for request validation,
 * response formatting, and data transformation.
 */

/**
 * Validate incoming request data
 *
 * @param {Object} data - Request body to validate
 * @returns {boolean} - True if valid, false otherwise
 */
function validateRequest(data) {
  if (!data || typeof data !== 'object') {
    return false;
  }

  // Check for required fields
  if (!data.hasOwnProperty('type')) {
    return false;
  }

  // Validate data size
  const dataStr = JSON.stringify(data);
  if (dataStr.length > 10000) {
    return false;
  }

  return true;
}

/**
 * Format response data for client consumption
 *
 * @param {Object} data - Raw response data
 * @returns {Object} - Formatted response
 */
function formatResponse(data) {
  return {
    success: true,
    data: data,
    timestamp: new Date().toISOString(),
    version: '1.0.0',
  };
}

/**
 * Calculate statistics from an array of numbers
 *
 * @param {Array<number>} numbers - Array of numbers
 * @returns {Object} - Statistics object
 */
function calculateStats(numbers) {
  if (!Array.isArray(numbers) || numbers.length === 0) {
    return {
      count: 0,
      sum: 0,
      avg: 0,
      min: null,
      max: null,
    };
  }

  const sum = numbers.reduce((acc, val) => acc + val, 0);
  const avg = sum / numbers.length;
  const min = Math.min(...numbers);
  const max = Math.max(...numbers);

  return {
    count: numbers.length,
    sum,
    avg,
    min,
    max,
  };
}

/**
 * Transform data for display
 *
 * @param {Object} rawData - Raw data object
 * @returns {Object} - Transformed data
 */
function transformData(rawData) {
  const transformed = {};

  for (const [key, value] of Object.entries(rawData)) {
    // Convert snake_case to camelCase
    const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());

    // Transform value based on type
    if (typeof value === 'string' && value.match(/^\d{4}-\d{2}-\d{2}/)) {
      transformed[camelKey] = new Date(value);
    } else {
      transformed[camelKey] = value;
    }
  }

  return transformed;
}

module.exports = {
  validateRequest,
  formatResponse,
  calculateStats,
  transformData,
};
