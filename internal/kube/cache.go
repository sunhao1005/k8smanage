package kube

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewCache 建立只读 informer 缓存（直接用 controller-runtime/pkg/cache，
// 不引入完整 Manager，更轻——评审 #5）。返回的 cache.Cache 实现 client.Reader。
// 调用方负责在 ctx 取消时停止；本函数会阻塞等待首次同步完成。
func NewCache(ctx context.Context, cfg *rest.Config) (cache.Cache, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	c, err := cache.New(cfg, cache.Options{
		Scheme: scheme,
		// 剥离 managedFields：缓存里用不上、却常占大量内存，降低每对象内存占用。
		DefaultTransform: stripManagedFields,
	})
	if err != nil {
		return nil, fmt.Errorf("new cache: %w", err)
	}
	go func() {
		if err := c.Start(ctx); err != nil {
			// Start 在 ctx 取消时正常返回 nil；非 nil 才是异常。
			fmt.Printf("cache stopped: %v\n", err)
		}
	}()
	// 首次同步加超时，避免 kubeconfig 指向不可达端点时启动卡死。
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !c.WaitForCacheSync(syncCtx) {
		return nil, fmt.Errorf("等待缓存同步超时/失败")
	}
	return c, nil
}

// stripManagedFields 在对象进缓存前清掉 metadata.managedFields（我们读不到也用不上，省内存）。
func stripManagedFields(i interface{}) (interface{}, error) {
	if o, ok := i.(metav1.Object); ok {
		o.SetManagedFields(nil)
	}
	return i, nil
}

// 确保 cache.Cache 满足 client.Reader（编译期校验）。
var _ client.Reader = (cache.Cache)(nil)
