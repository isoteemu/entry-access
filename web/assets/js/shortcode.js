let emojiList = [];
let emojiLoadPromise = null;

/**
 * Loads emoji data from the JSON file
 * @returns {Promise<void>}
 */
async function loadEmojis() {
    try {
        const response = await fetch('/dist/assets/sas-emoji.json');
        if (!response.ok) {
            throw new Error(`Failed to fetch emoji file: ${response.status}`);
        }
        
        emojiList = await response.json();
        
        // Make sure 64 emojis are available
        if (emojiList.length !== 64) {
            throw new Error(`Not enough emojis available: ${emojiList.length}`);
        }
    } catch (error) {
        throw new Error(`Failed to load emojis: ${error.message}`);
    }
}

/**
 * Converts a UUID string to a compressed 6-emoji SAS code
 * The UUID is hashed with SHA-256 and the first 36 bits are used to generate 6 emojis
 * @param {string} uuid - The UUID string to convert
 * @returns {Promise<string>} The emoji SAS code
 */
async function UUIDtoSAS(uuid) {
    // Wait for emojis to be loaded
    await emojiLoadPromise;

    // Validate UUID is not empty
    if (!uuid || uuid.length === 0) {
        throw new Error(`Invalid UUID: cannot be empty`);
    }

    // Convert UUID string to bytes for hashing
    const uuidBytes = new TextEncoder().encode(uuid);

    // Hash the UUID bytes using Web Crypto API
    const hashBuffer = await crypto.subtle.digest('SHA-256', uuidBytes);
    const hash = new Uint8Array(hashBuffer);

    // Extract first 36 bits (6 emojis × 6 bits each) from the hash
    let result = '';

    for (let i = 0; i < 6; i++) {
        // Extract 6 bits starting at bit position (i * 6)
        const value = extract6BitsFromHash(hash, i * 6);
        
        // Map to emoji (6 bits = 0-63 range, perfect for 64 emojis)
        result += emojiList[value].emoji;
    }

    return result;
}

/**
 * Extracts 6 bits from a hash starting at the given bit offset
 * @param {Uint8Array} hash - The hash byte array
 * @param {number} bitOffset - The bit offset to start extraction from
 * @returns {number} The extracted 6-bit value (0-63)
 */
function extract6BitsFromHash(hash, bitOffset) {
    const byteIndex = Math.floor(bitOffset / 8);
    const bitInByte = bitOffset % 8;

    let value = 0;

    // Handle case where 6 bits span across byte boundaries
    if (bitInByte <= 2) {
        // All 6 bits fit within the current byte
        value = (hash[byteIndex] >> (2 - bitInByte)) & 0x3F;
    } else {
        // 6 bits span across two bytes
        const bitsFromFirstByte = 8 - bitInByte;
        const bitsFromSecondByte = 6 - bitsFromFirstByte;

        value = hash[byteIndex] & ((1 << bitsFromFirstByte) - 1);
        value <<= bitsFromSecondByte;

        if (byteIndex + 1 < hash.length) {
            value |= hash[byteIndex + 1] >> (8 - bitsFromSecondByte);
        }
    }

    return value & 0x3F; // Ensure we only have 6 bits
}

/**
 * Verifies if an emoji SAS code corresponds to the given UUID
 * @param {string} uuid - The UUID to verify against
 * @param {string} sasCode - The emoji SAS code to verify
 * @returns {Promise<boolean>} True if the SAS code matches the UUID
 */
async function VerifySAS(uuid, sasCode) {
    try {
        const expectedSAS = await UUIDtoSAS(uuid);
        return expectedSAS === sasCode;
    } catch (error) {
        throw error;
    }
}

// Start loading emojis immediately when script is imported
emojiLoadPromise = loadEmojis();

export { UUIDtoSAS, VerifySAS, loadEmojis };
