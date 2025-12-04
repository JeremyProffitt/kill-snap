#!/bin/bash
# Test script to verify deployment is working correctly
# Run this after deployment to catch common issues

set -e

SITE_URL="${SITE_URL:-https://kill-snap.jeremy.ninja}"
API_URL="${API_URL:-$SITE_URL}"
FAILED=0

echo "=============================================="
echo "Kill Snap Deployment Test"
echo "Site URL: $SITE_URL"
echo "=============================================="
echo ""

# Test 1: Check if site is accessible
echo "Test 1: Site accessibility..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SITE_URL")
if [ "$HTTP_CODE" = "200" ]; then
    echo "  ✓ Site is accessible (HTTP $HTTP_CODE)"
else
    echo "  ✗ Site returned HTTP $HTTP_CODE"
    FAILED=1
fi

# Test 2: Check for JavaScript bundle
echo "Test 2: JavaScript bundle..."
JS_CHECK=$(curl -s "$SITE_URL" | grep -o 'static/js/main\.[a-z0-9]*\.js' | head -1)
if [ -n "$JS_CHECK" ]; then
    JS_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SITE_URL/$JS_CHECK")
    if [ "$JS_CODE" = "200" ]; then
        echo "  ✓ JavaScript bundle found and accessible"
    else
        echo "  ✗ JavaScript bundle returned HTTP $JS_CODE"
        FAILED=1
    fi
else
    echo "  ✗ JavaScript bundle not found in HTML"
    FAILED=1
fi

# Test 3: Check manifest.json
echo "Test 3: Manifest.json..."
MANIFEST_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SITE_URL/manifest.json")
if [ "$MANIFEST_CODE" = "200" ]; then
    # Check manifest doesn't reference missing files
    MANIFEST=$(curl -s "$SITE_URL/manifest.json")
    if echo "$MANIFEST" | grep -q "logo192.png"; then
        LOGO_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SITE_URL/logo192.png")
        if [ "$LOGO_CODE" != "200" ]; then
            echo "  ✗ Manifest references logo192.png but file returns HTTP $LOGO_CODE"
            FAILED=1
        else
            echo "  ✓ Manifest.json valid with all icons accessible"
        fi
    else
        echo "  ✓ Manifest.json valid (no missing icon references)"
    fi
else
    echo "  ✗ Manifest.json returned HTTP $MANIFEST_CODE"
    FAILED=1
fi

# Test 4: Check API OPTIONS (CORS preflight)
echo "Test 4: API CORS preflight..."
OPTIONS_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X OPTIONS "$API_URL/api/images" \
    -H "Origin: $SITE_URL" \
    -H "Access-Control-Request-Method: GET")
if [ "$OPTIONS_CODE" = "200" ]; then
    echo "  ✓ API CORS preflight working (HTTP $OPTIONS_CODE)"
else
    echo "  ✗ API CORS preflight returned HTTP $OPTIONS_CODE"
    FAILED=1
fi

# Test 5: Check API returns proper error for unauthorized requests
echo "Test 5: API unauthorized response..."
UNAUTH_RESPONSE=$(curl -s "$API_URL/api/images")
UNAUTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/api/images")
if [ "$UNAUTH_CODE" = "401" ]; then
    echo "  ✓ API returns 401 for unauthorized requests"
elif [ "$UNAUTH_CODE" = "500" ]; then
    echo "  ✗ API returns 500 - server error!"
    echo "     Response: $UNAUTH_RESPONSE"
    FAILED=1
else
    echo "  ? API returns HTTP $UNAUTH_CODE (expected 401)"
fi

# Test 6: Check API login endpoint
echo "Test 6: API login endpoint..."
LOGIN_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API_URL/api/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"test","password":"test"}')
if [ "$LOGIN_CODE" = "401" ] || [ "$LOGIN_CODE" = "200" ]; then
    echo "  ✓ API login endpoint responding (HTTP $LOGIN_CODE)"
else
    echo "  ✗ API login endpoint returned HTTP $LOGIN_CODE"
    FAILED=1
fi

# Test 7: Check CloudFront CDN for images
echo "Test 7: Image CDN accessibility..."
CDN_URL=$(curl -s "$SITE_URL" | grep -o 'https://[a-z0-9]*\.cloudfront\.net' | head -1)
if [ -n "$CDN_URL" ]; then
    echo "  ✓ CDN URL found in frontend: $CDN_URL"
else
    echo "  ? No CloudFront CDN URL found in frontend (may use direct S3)"
fi

# Test 8: Check favicon
echo "Test 8: Favicon..."
FAVICON_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SITE_URL/favicon.ico")
if [ "$FAVICON_CODE" = "200" ]; then
    echo "  ✓ Favicon accessible"
else
    echo "  ✗ Favicon returned HTTP $FAVICON_CODE"
    FAILED=1
fi

echo ""
echo "=============================================="
if [ "$FAILED" = "0" ]; then
    echo "All tests passed! ✓"
    exit 0
else
    echo "Some tests failed! ✗"
    exit 1
fi
