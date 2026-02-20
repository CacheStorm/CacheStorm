#!/bin/bash
# CacheStorm Tag-Based Invalidation Examples
# The killer feature of CacheStorm!

echo "=== Tag-Based Invalidation Examples ==="

echo ""
echo "1. Basic Tag Usage"
echo "==================="
redis-cli -p 6380 SETTAG user:1 "John Doe" users profile
redis-cli -p 6380 SETTAG user:2 "Jane Smith" users profile
redis-cli -p 6380 SETTAG user:3 "Bob Wilson" users settings
redis-cli -p 6380 SETTAG product:1 "Widget" products catalog
redis-cli -p 6380 SETTAG product:2 "Gadget" products catalog featured

echo ""
echo "Get all keys with 'users' tag:"
redis-cli -p 6380 TAGKEYS users

echo ""
echo "Count keys with 'products' tag:"
redis-cli -p 6380 TAGCOUNT products

echo ""
echo "2. API Response Caching"
echo "======================="
# Cache API responses with endpoint tags
redis-cli -p 6380 SETTAG "api:/users/123" '{"id":123,"name":"John"}' "api" "user:123"
redis-cli -p 6380 SETTAG "api:/users/123/profile" '{"bio":"..."}' "api" "user:123"
redis-cli -p 6380 SETTAG "api:/users/123/settings" '{"theme":"dark"}' "api" "user:123"

echo ""
echo "All API cache keys:"
redis-cli -p 6380 TAGKEYS api

echo ""
echo "Keys for specific user:"
redis-cli -p 6380 TAGKEYS "user:123"

echo ""
echo "Invalidate all cache for user 123:"
redis-cli -p 6380 INVALIDATE "user:123"
redis-cli -p 6380 TAGKEYS "user:123"

echo ""
echo "3. Session Management"
echo "====================="
redis-cli -p 6380 SETTAG "session:abc123" "user_data_1" "session" "user:456"
redis-cli -p 6380 SETTAG "session:def456" "user_data_2" "session" "user:456"
redis-cli -p 6380 SETTAG "session:ghi789" "user_data_3" "session" "user:789"

echo ""
echo "All sessions for user 456:"
redis-cli -p 6380 TAGKEYS "user:456"

echo ""
echo "Invalidate all sessions for user 456:"
redis-cli -p 6380 INVALIDATE "user:456"
redis-cli -p 6380 TAGKEYS "user:456"

echo ""
echo "4. E-commerce Example"
echo "====================="
# Product pages with category tags
redis-cli -p 6380 SETTAG "page:product:1" "<html>Product 1</html>" "page" "category:electronics" "brand:apple"
redis-cli -p 6380 SETTAG "page:product:2" "<html>Product 2</html>" "page" "category:electronics" "brand:samsung"
redis-cli -p 6380 SETTAG "page:product:3" "<html>Product 3</html>" "page" "category:clothing" "brand:nike"

echo ""
echo "All electronics pages:"
redis-cli -p 6380 TAGKEYS "category:electronics"

echo ""
echo "All Apple products:"
redis-cli -p 6380 TAGKEYS "brand:apple"

echo ""
echo "Invalidate all electronics pages:"
redis-cli -p 6380 INVALIDATE "category:electronics"
redis-cli -p 6380 TAGKEYS "category:electronics"

echo ""
echo "5. Multi-Tenant SaaS Example"
echo "============================="
# Each tenant's data is tagged with tenant ID
redis-cli -p 6380 SETTAG "tenant:acme:config" "acme_config" "tenant:acme"
redis-cli -p 6380 SETTAG "tenant:acme:users" "acme_users" "tenant:acme"
redis-cli -p 6380 SETTAG "tenant:acme:settings" "acme_settings" "tenant:acme"
redis-cli -p 6380 SETTAG "tenant:globex:config" "globex_config" "tenant:globex"
redis-cli -p 6380 SETTAG "tenant:globex:users" "globex_users" "tenant:globex"

echo ""
echo "All Acme Corp data:"
redis-cli -p 6380 TAGKEYS "tenant:acme"

echo ""
echo "Invalidate all Acme Corp cache:"
redis-cli -p 6380 INVALIDATE "tenant:acme"

echo ""
echo "6. Content Management System"
echo "============================"
# Cache pages, posts, comments with content tags
redis-cli -p 6380 SETTAG "cms:page:home" "homepage_content" "cms" "page" "layout:main"
redis-cli -p 6380 SETTAG "cms:page:about" "about_content" "cms" "page" "layout:main"
redis-cli -p 6380 SETTAG "cms:post:1" "blog_post_1" "cms" "post" "author:john"
redis-cli -p 6380 SETTAG "cms:post:2" "blog_post_2" "cms" "post" "author:jane"
redis-cli -p 6380 SETTAG "cms:comment:1" "comment_data" "cms" "comment" "post:1"

echo ""
echo "All CMS cache:"
redis-cli -p 6380 TAGKEYS cms

echo ""
echo "All blog posts:"
redis-cli -p 6380 TAGKEYS post

echo ""
echo "All John's content:"
redis-cli -p 6380 TAGKEYS "author:john"

echo ""
echo "Invalidate all posts:"
redis-cli -p 6380 INVALIDATE post

echo ""
echo "7. CDN Edge Caching"
echo "==================="
# Cache static assets with URL patterns
redis-cli -p 6380 SETTAG "cdn:/static/js/app.js" "minified_js" "cdn" "static" "js"
redis-cli -p 6380 SETTAG "cdn:/static/css/style.css" "minified_css" "cdn" "static" "css"
redis-cli -p 6380 SETTAG "cdn:/images/logo.png" "logo_image" "cdn" "static" "images"

echo ""
echo "All CDN cache:"
redis-cli -p 6380 TAGKEYS cdn

echo ""
echo "All static files:"
redis-cli -p 6380 TAGKEYS static

echo ""
echo "Purge all CSS files:"
redis-cli -p 6380 INVALIDATE css

echo ""
echo "8. Tag Information"
echo "=================="
redis-cli -p 6380 TAGINFO cdn
redis-cli -p 6380 TAGCOUNT static

echo ""
echo "9. Cleanup"
echo "=========="
redis-cli -p 6380 FLUSHDB
echo "All done!"
