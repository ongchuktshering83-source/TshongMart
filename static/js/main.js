// Tshongmart Main JavaScript

function loadProducts() {
    console.log('Loading products...');
    
    fetch('/api/products')
        .then(function(response) {
            console.log('Response status:', response.status);
            if (!response.ok) {
                throw new Error('HTTP error ' + response.status);
            }
            return response.json();
        })
        .then(function(data) {
            console.log('Data received:', data);
            
            // Handle null or undefined response
            var products = data || [];
            
            // Make sure products is an array
            if (!Array.isArray(products)) {
                console.error('Products is not an array:', products);
                products = [];
            }
            
            var container = document.getElementById('products-container');
            if (!container) return;
            
            if (products.length === 0) {
                container.innerHTML = '<div class="error" style="text-align:center; padding:40px;">No products available. <a href="/dashboard">Add some products</a></div>';
                return;
            }
            
            var html = '';
            for (var i = 0; i < products.length; i++) {
                var p = products[i];
                html += '<div class="product-card">';
                html += '<h3>' + escapeHtml(p.title || 'No title') + '</h3>';
                html += '<p>' + escapeHtml((p.description || '').substring(0, 100)) + '...</p>';
                html += '<div class="price">Nu. ' + (p.price || 0).toLocaleString() + '</div>';
                html += '<span class="condition">' + escapeHtml(p.condition || 'Not specified') + '</span>';
                html += '<p><strong>Seller:</strong> ' + escapeHtml(p.seller || 'Unknown') + '</p>';
                html += '<button class="btn-primary" onclick="addToCart(' + p.id + ')">Add to Cart 🛒</button>';
                html += '</div>';
            }
            container.innerHTML = html;
        })
        .catch(function(error) {
            console.error('Error loading products:', error);
            var container = document.getElementById('products-container');
            if (container) {
                container.innerHTML = '<div class="error" style="text-align:center; padding:40px; color:red;">Error loading products. Please refresh the page.</div>';
            }
        });
}

function addToCart(productId) {
    fetch('/api/cart/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ product_id: productId, quantity: 1 })
    })
    .then(function(res) { return res.json(); })
    .then(function() { alert('✅ Added to cart!'); })
    .catch(function() { alert('Please login to add items to cart'); });
}

function escapeHtml(text) {
    if (!text) return '';
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Load products when page loads
if (document.getElementById('products-container')) {
    loadProducts();
}
