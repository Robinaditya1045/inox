#!/usr/bin/env bash
# ==============================================================================
# Inox Environment IP Toggle Script
# Toggles between 'localhost' and the IP address specified in 'ip.txt' across
# frontend/.env, admin-portal/.env, backend/.env, and vite.config.ts.
# ==============================================================================

IP_FILE="ip.txt"

# 1. Ensure ip.txt exists and read the IP
if [ ! -f "$IP_FILE" ]; then
    echo "⚠️  '$IP_FILE' not found in root directory!"
    DETECTED_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
    if [ -z "$DETECTED_IP" ]; then
        DETECTED_IP="192.168.1.100"
    fi
    echo "Creating '$IP_FILE' with auto-detected local IP: $DETECTED_IP"
    echo "$DETECTED_IP" > "$IP_FILE"
fi

TARGET_IP=$(cat "$IP_FILE" | tr -d '[:space:]')

if [ -z "$TARGET_IP" ]; then
    echo "❌ Error: '$IP_FILE' is empty. Please enter your LAN IP (e.g. 192.168.1.100) in '$IP_FILE'."
    exit 1
fi

# Files to inspect and toggle
FILES=(
    "frontend/.env"
    "admin-portal/.env"
    "backend/.env"
    "admin-portal/vite.config.ts"
)

# 2. Check current state by inspecting frontend/.env or admin-portal/.env
if grep -q "http://localhost:8080" "frontend/.env" 2>/dev/null || grep -q "http://localhost:8080" "admin-portal/.env" 2>/dev/null; then
    CURRENT_STATE="localhost"
    NEW_STATE="$TARGET_IP"
    MODE_DESC="Local IP ($TARGET_IP)"
else
    CURRENT_STATE="$TARGET_IP"
    NEW_STATE="localhost"
    MODE_DESC="Localhost (localhost)"
    # If the file had some other IP previously, detect what IP is currently in frontend/.env
    PREV_IP=$(grep -oE "http://[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:8080" "frontend/.env" 2>/dev/null | head -1 | sed 's|http://||; s|:8080||')
    if [ -n "$PREV_IP" ] && [ "$PREV_IP" != "localhost" ]; then
        CURRENT_STATE="$PREV_IP"
    fi
fi

echo "========================================================================"
echo "🔄 Inox Environment IP Toggle"
echo "------------------------------------------------------------------------"
echo "Current State : $CURRENT_STATE"
echo "Target State  : $NEW_STATE ($MODE_DESC)"
echo "========================================================================"

# 3. Perform the toggle across files
UPDATED_COUNT=0
for FILE in "${FILES[@]}"; do
    if [ -f "$FILE" ]; then
        # Use sed to replace CURRENT_STATE with NEW_STATE for URLs and IPs
        # We specifically replace across common endpoints (http://, ws://, or raw IP/hostname assignments)
        if [ "$CURRENT_STATE" = "localhost" ]; then
            # Switching from localhost -> TARGET_IP
            # For backend/.env we also append the new IP to CORS_ALLOWED_ORIGINS if not present
            sed -i "s|localhost:8080|${NEW_STATE}:8080|g" "$FILE"
            sed -i "s|localhost:9000|${NEW_STATE}:9000|g" "$FILE"
            sed -i "s|localhost:9001|${NEW_STATE}:9001|g" "$FILE"
            if [ "$FILE" = "backend/.env" ]; then
                # Ensure new IP origins are allowed in CORS
                sed -i "s|CORS_ALLOWED_ORIGINS=.*|CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:5174,http://localhost:3000,http://${NEW_STATE}:5173,http://${NEW_STATE}:5174,http://${NEW_STATE}:3000|g" "$FILE"
            fi
        else
            # Switching from TARGET_IP (or PREV_IP) -> localhost
            sed -i "s|${CURRENT_STATE}:8080|localhost:8080|g" "$FILE"
            sed -i "s|${CURRENT_STATE}:9000|localhost:9000|g" "$FILE"
            sed -i "s|${CURRENT_STATE}:9001|localhost:9001|g" "$FILE"
            if [ "$FILE" = "backend/.env" ]; then
                sed -i "s|CORS_ALLOWED_ORIGINS=.*|CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:5174,http://localhost:3000|g" "$FILE"
            fi
        fi
        echo "  ✔️ Updated: $FILE"
        UPDATED_COUNT=$((UPDATED_COUNT + 1))
    else
        echo "  ⚠️ Skipped: $FILE (file not found)"
    fi
done

echo "------------------------------------------------------------------------"
if [ "$NEW_STATE" = "localhost" ]; then
    echo "✨ Switched $UPDATED_COUNT files to LOCALHOST mode."
else
    echo "✨ Switched $UPDATED_COUNT files to NETWORK IP mode ($TARGET_IP)."
    echo "💡 Note: Make sure your frontend/admin-portal dev servers bind to 0.0.0.0"
    echo "   (using 'npm run dev -- --host 0.0.0.0' or via Makefile/dev.sh)."
fi
echo "========================================================================"
