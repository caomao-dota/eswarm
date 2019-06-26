package wiredtiger

/*
#cgo CFLAGS: -I../wdlibs/libs/include
#cgo LDFLAGS: ${SRCDIR}/../wdlibs/libs/lib/libwiredtiger.a

#include <wiredtiger.h>
#include <errno.h>

*/
import "C"

// Error returns

const (
	WT_ROLLBACK      = int(C.WT_ROLLBACK)
	WT_DUPLICATE_KEY = int(C.WT_DUPLICATE_KEY)
	WT_ERROR         = int(C.WT_ERROR)
	WT_NOTFOUND      = int(C.WT_NOTFOUND)
	WT_PANIC         = int(C.WT_PANIC)
	WT_RUN_RECOVERY  = int(C.WT_RUN_RECOVERY)
	EINVAL           = int(C.EINVAL)
)

// Connection statistics
const (
	WT_STAT_CONN_LSM_WORK_QUEUE_APP               = int(C.WT_STAT_CONN_LSM_WORK_QUEUE_APP)
	WT_STAT_CONN_LSM_WORK_QUEUE_MANAGER           = int(C.WT_STAT_CONN_LSM_WORK_QUEUE_MANAGER)
	WT_STAT_CONN_LSM_ROWS_MERGED                  = int(C.WT_STAT_CONN_LSM_ROWS_MERGED)
	WT_STAT_CONN_LSM_CHECKPOINT_THROTTLE          = int(C.WT_STAT_CONN_LSM_CHECKPOINT_THROTTLE)
	WT_STAT_CONN_LSM_MERGE_THROTTLE               = int(C.WT_STAT_CONN_LSM_MERGE_THROTTLE)
	WT_STAT_CONN_LSM_WORK_QUEUE_SWITCH            = int(C.WT_STAT_CONN_LSM_WORK_QUEUE_SWITCH)
	WT_STAT_CONN_LSM_WORK_UNITS_DISCARDED         = int(C.WT_STAT_CONN_LSM_WORK_UNITS_DISCARDED)
	WT_STAT_CONN_LSM_WORK_UNITS_DONE              = int(C.WT_STAT_CONN_LSM_WORK_UNITS_DONE)
	WT_STAT_CONN_LSM_WORK_UNITS_CREATED           = int(C.WT_STAT_CONN_LSM_WORK_UNITS_CREATED)
	WT_STAT_CONN_LSM_WORK_QUEUE_MAX               = int(C.WT_STAT_CONN_LSM_WORK_QUEUE_MAX)
	WT_STAT_CONN_ASYNC_CUR_QUEUE                  = int(C.WT_STAT_CONN_ASYNC_CUR_QUEUE)
	WT_STAT_CONN_ASYNC_MAX_QUEUE                  = int(C.WT_STAT_CONN_ASYNC_MAX_QUEUE)
	WT_STAT_CONN_ASYNC_ALLOC_RACE                 = int(C.WT_STAT_CONN_ASYNC_ALLOC_RACE)
	WT_STAT_CONN_ASYNC_FLUSH                      = int(C.WT_STAT_CONN_ASYNC_FLUSH)
	WT_STAT_CONN_ASYNC_ALLOC_VIEW                 = int(C.WT_STAT_CONN_ASYNC_ALLOC_VIEW)
	WT_STAT_CONN_ASYNC_FULL                       = int(C.WT_STAT_CONN_ASYNC_FULL)
	WT_STAT_CONN_ASYNC_NOWORK                     = int(C.WT_STAT_CONN_ASYNC_NOWORK)
	WT_STAT_CONN_ASYNC_OP_ALLOC                   = int(C.WT_STAT_CONN_ASYNC_OP_ALLOC)
	WT_STAT_CONN_ASYNC_OP_COMPACT                 = int(C.WT_STAT_CONN_ASYNC_OP_COMPACT)
	WT_STAT_CONN_ASYNC_OP_INSERT                  = int(C.WT_STAT_CONN_ASYNC_OP_INSERT)
	WT_STAT_CONN_ASYNC_OP_REMOVE                  = int(C.WT_STAT_CONN_ASYNC_OP_REMOVE)
	WT_STAT_CONN_ASYNC_OP_SEARCH                  = int(C.WT_STAT_CONN_ASYNC_OP_SEARCH)
	WT_STAT_CONN_ASYNC_OP_UPDATE                  = int(C.WT_STAT_CONN_ASYNC_OP_UPDATE)
	WT_STAT_CONN_BLOCK_PRELOAD                    = int(C.WT_STAT_CONN_BLOCK_PRELOAD)
	WT_STAT_CONN_BLOCK_READ                       = int(C.WT_STAT_CONN_BLOCK_READ)
	WT_STAT_CONN_BLOCK_WRITE                      = int(C.WT_STAT_CONN_BLOCK_WRITE)
	WT_STAT_CONN_BLOCK_BYTE_READ                  = int(C.WT_STAT_CONN_BLOCK_BYTE_READ)
	WT_STAT_CONN_BLOCK_BYTE_WRITE                 = int(C.WT_STAT_CONN_BLOCK_BYTE_WRITE)
	WT_STAT_CONN_BLOCK_MAP_READ                   = int(C.WT_STAT_CONN_BLOCK_MAP_READ)
	WT_STAT_CONN_BLOCK_BYTE_MAP_READ              = int(C.WT_STAT_CONN_BLOCK_BYTE_MAP_READ)
	WT_STAT_CONN_CACHE_BYTES_INUSE                = int(C.WT_STAT_CONN_CACHE_BYTES_INUSE)
	WT_STAT_CONN_CACHE_BYTES_READ                 = int(C.WT_STAT_CONN_CACHE_BYTES_READ)
	WT_STAT_CONN_CACHE_BYTES_WRITE                = int(C.WT_STAT_CONN_CACHE_BYTES_WRITE)
	WT_STAT_CONN_CACHE_EVICTION_CHECKPOINT        = int(C.WT_STAT_CONN_CACHE_EVICTION_CHECKPOINT)
	WT_STAT_CONN_CACHE_EVICTION_AGGRESSIVE_SET    = int(C.WT_STAT_CONN_CACHE_EVICTION_AGGRESSIVE_SET)
	WT_STAT_CONN_CACHE_EVICTION_QUEUE_EMPTY       = int(C.WT_STAT_CONN_CACHE_EVICTION_QUEUE_EMPTY)
	WT_STAT_CONN_CACHE_EVICTION_QUEUE_NOT_EMPTY   = int(C.WT_STAT_CONN_CACHE_EVICTION_QUEUE_NOT_EMPTY)
	WT_STAT_CONN_CACHE_EVICTION_SERVER_EVICTING   = int(C.WT_STAT_CONN_CACHE_EVICTION_SERVER_EVICTING)
	WT_STAT_CONN_CACHE_EVICTION_SLOW              = int(C.WT_STAT_CONN_CACHE_EVICTION_SLOW)
	WT_STAT_CONN_CACHE_EVICTION_WORKER_EVICTING   = int(C.WT_STAT_CONN_CACHE_EVICTION_WORKER_EVICTING)
	WT_STAT_CONN_CACHE_EVICTION_FORCE_FAIL        = int(C.WT_STAT_CONN_CACHE_EVICTION_FORCE_FAIL)
	WT_STAT_CONN_CACHE_EVICTION_HAZARD            = int(C.WT_STAT_CONN_CACHE_EVICTION_HAZARD)
	WT_STAT_CONN_CACHE_INMEM_SPLITTABLE           = int(C.WT_STAT_CONN_CACHE_INMEM_SPLITTABLE)
	WT_STAT_CONN_CACHE_INMEM_SPLIT                = int(C.WT_STAT_CONN_CACHE_INMEM_SPLIT)
	WT_STAT_CONN_CACHE_EVICTION_INTERNAL          = int(C.WT_STAT_CONN_CACHE_EVICTION_INTERNAL)
	WT_STAT_CONN_CACHE_EVICTION_SPLIT_INTERNAL    = int(C.WT_STAT_CONN_CACHE_EVICTION_SPLIT_INTERNAL)
	WT_STAT_CONN_CACHE_EVICTION_SPLIT_LEAF        = int(C.WT_STAT_CONN_CACHE_EVICTION_SPLIT_LEAF)
	WT_STAT_CONN_CACHE_LOOKASIDE_INSERT           = int(C.WT_STAT_CONN_CACHE_LOOKASIDE_INSERT)
	WT_STAT_CONN_CACHE_LOOKASIDE_REMOVE           = int(C.WT_STAT_CONN_CACHE_LOOKASIDE_REMOVE)
	WT_STAT_CONN_CACHE_BYTES_MAX                  = int(C.WT_STAT_CONN_CACHE_BYTES_MAX)
	WT_STAT_CONN_CACHE_EVICTION_MAXIMUM_PAGE_SIZE = int(C.WT_STAT_CONN_CACHE_EVICTION_MAXIMUM_PAGE_SIZE)
	WT_STAT_CONN_CACHE_EVICTION_DIRTY             = int(C.WT_STAT_CONN_CACHE_EVICTION_DIRTY)
	WT_STAT_CONN_CACHE_EVICTION_DEEPEN            = int(C.WT_STAT_CONN_CACHE_EVICTION_DEEPEN)
	WT_STAT_CONN_CACHE_WRITE_LOOKASIDE            = int(C.WT_STAT_CONN_CACHE_WRITE_LOOKASIDE)
	WT_STAT_CONN_CACHE_PAGES_INUSE                = int(C.WT_STAT_CONN_CACHE_PAGES_INUSE)
	WT_STAT_CONN_CACHE_EVICTION_FORCE             = int(C.WT_STAT_CONN_CACHE_EVICTION_FORCE)
	WT_STAT_CONN_CACHE_EVICTION_FORCE_DELETE      = int(C.WT_STAT_CONN_CACHE_EVICTION_FORCE_DELETE)
	WT_STAT_CONN_CACHE_EVICTION_APP               = int(C.WT_STAT_CONN_CACHE_EVICTION_APP)
	WT_STAT_CONN_CACHE_READ                       = int(C.WT_STAT_CONN_CACHE_READ)
	WT_STAT_CONN_CACHE_READ_LOOKASIDE             = int(C.WT_STAT_CONN_CACHE_READ_LOOKASIDE)
	WT_STAT_CONN_CACHE_EVICTION_FAIL              = int(C.WT_STAT_CONN_CACHE_EVICTION_FAIL)
	WT_STAT_CONN_CACHE_EVICTION_WALK              = int(C.WT_STAT_CONN_CACHE_EVICTION_WALK)
	WT_STAT_CONN_CACHE_WRITE                      = int(C.WT_STAT_CONN_CACHE_WRITE)
	WT_STAT_CONN_CACHE_WRITE_RESTORE              = int(C.WT_STAT_CONN_CACHE_WRITE_RESTORE)
	WT_STAT_CONN_CACHE_OVERHEAD                   = int(C.WT_STAT_CONN_CACHE_OVERHEAD)
	WT_STAT_CONN_CACHE_BYTES_INTERNAL             = int(C.WT_STAT_CONN_CACHE_BYTES_INTERNAL)
	WT_STAT_CONN_CACHE_BYTES_LEAF                 = int(C.WT_STAT_CONN_CACHE_BYTES_LEAF)
	WT_STAT_CONN_CACHE_BYTES_DIRTY                = int(C.WT_STAT_CONN_CACHE_BYTES_DIRTY)
	WT_STAT_CONN_CACHE_PAGES_DIRTY                = int(C.WT_STAT_CONN_CACHE_PAGES_DIRTY)
	WT_STAT_CONN_CACHE_EVICTION_CLEAN             = int(C.WT_STAT_CONN_CACHE_EVICTION_CLEAN)
	WT_STAT_CONN_COND_AUTO_WAIT_RESET             = int(C.WT_STAT_CONN_COND_AUTO_WAIT_RESET)
	WT_STAT_CONN_COND_AUTO_WAIT                   = int(C.WT_STAT_CONN_COND_AUTO_WAIT)
	WT_STAT_CONN_FILE_OPEN                        = int(C.WT_STAT_CONN_FILE_OPEN)
	WT_STAT_CONN_MEMORY_ALLOCATION                = int(C.WT_STAT_CONN_MEMORY_ALLOCATION)
	WT_STAT_CONN_MEMORY_FREE                      = int(C.WT_STAT_CONN_MEMORY_FREE)
	WT_STAT_CONN_MEMORY_GROW                      = int(C.WT_STAT_CONN_MEMORY_GROW)
	WT_STAT_CONN_COND_WAIT                        = int(C.WT_STAT_CONN_COND_WAIT)
	WT_STAT_CONN_RWLOCK_READ                      = int(C.WT_STAT_CONN_RWLOCK_READ)
	WT_STAT_CONN_RWLOCK_WRITE                     = int(C.WT_STAT_CONN_RWLOCK_WRITE)
	WT_STAT_CONN_READ_IO                          = int(C.WT_STAT_CONN_READ_IO)
	WT_STAT_CONN_WRITE_IO                         = int(C.WT_STAT_CONN_WRITE_IO)
	WT_STAT_CONN_CURSOR_CREATE                    = int(C.WT_STAT_CONN_CURSOR_CREATE)
	WT_STAT_CONN_CURSOR_INSERT                    = int(C.WT_STAT_CONN_CURSOR_INSERT)
	WT_STAT_CONN_CURSOR_NEXT                      = int(C.WT_STAT_CONN_CURSOR_NEXT)
	WT_STAT_CONN_CURSOR_PREV                      = int(C.WT_STAT_CONN_CURSOR_PREV)
	WT_STAT_CONN_CURSOR_REMOVE                    = int(C.WT_STAT_CONN_CURSOR_REMOVE)
	WT_STAT_CONN_CURSOR_RESET                     = int(C.WT_STAT_CONN_CURSOR_RESET)
	WT_STAT_CONN_CURSOR_RESTART                   = int(C.WT_STAT_CONN_CURSOR_RESTART)
	WT_STAT_CONN_CURSOR_SEARCH                    = int(C.WT_STAT_CONN_CURSOR_SEARCH)
	WT_STAT_CONN_CURSOR_SEARCH_NEAR               = int(C.WT_STAT_CONN_CURSOR_SEARCH_NEAR)
	WT_STAT_CONN_CURSOR_UPDATE                    = int(C.WT_STAT_CONN_CURSOR_UPDATE)
	WT_STAT_CONN_CURSOR_TRUNCATE                  = int(C.WT_STAT_CONN_CURSOR_TRUNCATE)
	WT_STAT_CONN_DH_CONN_HANDLE_COUNT             = int(C.WT_STAT_CONN_DH_CONN_HANDLE_COUNT)
	WT_STAT_CONN_DH_SWEEP_REF                     = int(C.WT_STAT_CONN_DH_SWEEP_REF)
	WT_STAT_CONN_DH_SWEEP_CLOSE                   = int(C.WT_STAT_CONN_DH_SWEEP_CLOSE)
	WT_STAT_CONN_DH_SWEEP_REMOVE                  = int(C.WT_STAT_CONN_DH_SWEEP_REMOVE)
	WT_STAT_CONN_DH_SWEEP_TOD                     = int(C.WT_STAT_CONN_DH_SWEEP_TOD)
	WT_STAT_CONN_DH_SWEEPS                        = int(C.WT_STAT_CONN_DH_SWEEPS)
	WT_STAT_CONN_DH_SESSION_HANDLES               = int(C.WT_STAT_CONN_DH_SESSION_HANDLES)
	WT_STAT_CONN_DH_SESSION_SWEEPS                = int(C.WT_STAT_CONN_DH_SESSION_SWEEPS)
	WT_STAT_CONN_LOG_SLOT_SWITCH_BUSY             = int(C.WT_STAT_CONN_LOG_SLOT_SWITCH_BUSY)
	WT_STAT_CONN_LOG_SLOT_CLOSES                  = int(C.WT_STAT_CONN_LOG_SLOT_CLOSES)
	WT_STAT_CONN_LOG_SLOT_RACES                   = int(C.WT_STAT_CONN_LOG_SLOT_RACES)
	//	WT_STAT_CONN_LOG_SLOT_TRANSITIONS             = int(C.WT_STAT_CONN_LOG_SLOT_YIELD_RACE)
	//	WT_STAT_CONN_LOG_SLOT_JOINS                   = int(C.WT_STAT_CONN_LOG_SLOT_JOINS)
	WT_STAT_CONN_LOG_SLOT_UNBUFFERED       = int(C.WT_STAT_CONN_LOG_SLOT_UNBUFFERED)
	WT_STAT_CONN_LOG_BYTES_PAYLOAD         = int(C.WT_STAT_CONN_LOG_BYTES_PAYLOAD)
	WT_STAT_CONN_LOG_BYTES_WRITTEN         = int(C.WT_STAT_CONN_LOG_BYTES_WRITTEN)
	WT_STAT_CONN_LOG_ZERO_FILLS            = int(C.WT_STAT_CONN_LOG_ZERO_FILLS)
	WT_STAT_CONN_LOG_FLUSH                 = int(C.WT_STAT_CONN_LOG_FLUSH)
	WT_STAT_CONN_LOG_FORCE_WRITE           = int(C.WT_STAT_CONN_LOG_FORCE_WRITE)
	WT_STAT_CONN_LOG_FORCE_WRITE_SKIP      = int(C.WT_STAT_CONN_LOG_FORCE_WRITE_SKIP)
	WT_STAT_CONN_LOG_COMPRESS_WRITES       = int(C.WT_STAT_CONN_LOG_COMPRESS_WRITES)
	WT_STAT_CONN_LOG_COMPRESS_WRITE_FAILS  = int(C.WT_STAT_CONN_LOG_COMPRESS_WRITE_FAILS)
	WT_STAT_CONN_LOG_COMPRESS_SMALL        = int(C.WT_STAT_CONN_LOG_COMPRESS_SMALL)
	WT_STAT_CONN_LOG_RELEASE_WRITE_LSN     = int(C.WT_STAT_CONN_LOG_RELEASE_WRITE_LSN)
	WT_STAT_CONN_LOG_SCANS                 = int(C.WT_STAT_CONN_LOG_SCANS)
	WT_STAT_CONN_LOG_SCAN_REREADS          = int(C.WT_STAT_CONN_LOG_SCAN_REREADS)
	WT_STAT_CONN_LOG_WRITE_LSN             = int(C.WT_STAT_CONN_LOG_WRITE_LSN)
	WT_STAT_CONN_LOG_WRITE_LSN_SKIP        = int(C.WT_STAT_CONN_LOG_WRITE_LSN_SKIP)
	WT_STAT_CONN_LOG_SYNC                  = int(C.WT_STAT_CONN_LOG_SYNC)
	WT_STAT_CONN_LOG_SYNC_DIR              = int(C.WT_STAT_CONN_LOG_SYNC_DIR)
	WT_STAT_CONN_LOG_WRITES                = int(C.WT_STAT_CONN_LOG_WRITES)
	WT_STAT_CONN_LOG_SLOT_CONSOLIDATED     = int(C.WT_STAT_CONN_LOG_SLOT_CONSOLIDATED)
	WT_STAT_CONN_LOG_MAX_FILESIZE          = int(C.WT_STAT_CONN_LOG_MAX_FILESIZE)
	WT_STAT_CONN_LOG_PREALLOC_MAX          = int(C.WT_STAT_CONN_LOG_PREALLOC_MAX)
	WT_STAT_CONN_LOG_PREALLOC_MISSED       = int(C.WT_STAT_CONN_LOG_PREALLOC_MISSED)
	WT_STAT_CONN_LOG_PREALLOC_FILES        = int(C.WT_STAT_CONN_LOG_PREALLOC_FILES)
	WT_STAT_CONN_LOG_PREALLOC_USED         = int(C.WT_STAT_CONN_LOG_PREALLOC_USED)
	WT_STAT_CONN_LOG_SCAN_RECORDS          = int(C.WT_STAT_CONN_LOG_SCAN_RECORDS)
	WT_STAT_CONN_LOG_COMPRESS_MEM          = int(C.WT_STAT_CONN_LOG_COMPRESS_MEM)
	WT_STAT_CONN_LOG_BUFFER_SIZE           = int(C.WT_STAT_CONN_LOG_BUFFER_SIZE)
	WT_STAT_CONN_LOG_COMPRESS_LEN          = int(C.WT_STAT_CONN_LOG_COMPRESS_LEN)
	WT_STAT_CONN_LOG_SLOT_COALESCED        = int(C.WT_STAT_CONN_LOG_SLOT_COALESCED)
	WT_STAT_CONN_LOG_CLOSE_YIELDS          = int(C.WT_STAT_CONN_LOG_CLOSE_YIELDS)
	WT_STAT_CONN_REC_PAGE_DELETE_FAST      = int(C.WT_STAT_CONN_REC_PAGE_DELETE_FAST)
	WT_STAT_CONN_REC_PAGES                 = int(C.WT_STAT_CONN_REC_PAGES)
	WT_STAT_CONN_REC_PAGES_EVICTION        = int(C.WT_STAT_CONN_REC_PAGES_EVICTION)
	WT_STAT_CONN_REC_PAGE_DELETE           = int(C.WT_STAT_CONN_REC_PAGE_DELETE)
	WT_STAT_CONN_REC_SPLIT_STASHED_BYTES   = int(C.WT_STAT_CONN_REC_SPLIT_STASHED_BYTES)
	WT_STAT_CONN_REC_SPLIT_STASHED_OBJECTS = int(C.WT_STAT_CONN_REC_SPLIT_STASHED_OBJECTS)
	//	WT_STAT_CONN_SESSION_CURSOR_OPEN              = int(C.WT_STAT_CONN_SESSION_CURSOR_OPEN)
	WT_STAT_CONN_SESSION_OPEN                = int(C.WT_STAT_CONN_SESSION_OPEN)
	WT_STAT_CONN_PAGE_BUSY_BLOCKED           = int(C.WT_STAT_CONN_PAGE_BUSY_BLOCKED)
	WT_STAT_CONN_PAGE_FORCIBLE_EVICT_BLOCKED = int(C.WT_STAT_CONN_PAGE_FORCIBLE_EVICT_BLOCKED)
	WT_STAT_CONN_PAGE_LOCKED_BLOCKED         = int(C.WT_STAT_CONN_PAGE_LOCKED_BLOCKED)
	WT_STAT_CONN_PAGE_READ_BLOCKED           = int(C.WT_STAT_CONN_PAGE_READ_BLOCKED)
	WT_STAT_CONN_PAGE_SLEEP                  = int(C.WT_STAT_CONN_PAGE_SLEEP)
	WT_STAT_CONN_TXN_SNAPSHOTS_CREATED       = int(C.WT_STAT_CONN_TXN_SNAPSHOTS_CREATED)
	WT_STAT_CONN_TXN_SNAPSHOTS_DROPPED       = int(C.WT_STAT_CONN_TXN_SNAPSHOTS_DROPPED)
	WT_STAT_CONN_TXN_BEGIN                   = int(C.WT_STAT_CONN_TXN_BEGIN)
	WT_STAT_CONN_TXN_CHECKPOINT_RUNNING      = int(C.WT_STAT_CONN_TXN_CHECKPOINT_RUNNING)
	WT_STAT_CONN_TXN_CHECKPOINT_GENERATION   = int(C.WT_STAT_CONN_TXN_CHECKPOINT_GENERATION)
	WT_STAT_CONN_TXN_CHECKPOINT_TIME_MAX     = int(C.WT_STAT_CONN_TXN_CHECKPOINT_TIME_MAX)
	WT_STAT_CONN_TXN_CHECKPOINT_TIME_MIN     = int(C.WT_STAT_CONN_TXN_CHECKPOINT_TIME_MIN)
	WT_STAT_CONN_TXN_CHECKPOINT_TIME_RECENT  = int(C.WT_STAT_CONN_TXN_CHECKPOINT_TIME_RECENT)
	WT_STAT_CONN_TXN_CHECKPOINT_TIME_TOTAL   = int(C.WT_STAT_CONN_TXN_CHECKPOINT_TIME_TOTAL)
	WT_STAT_CONN_TXN_CHECKPOINT              = int(C.WT_STAT_CONN_TXN_CHECKPOINT)
	WT_STAT_CONN_TXN_FAIL_CACHE              = int(C.WT_STAT_CONN_TXN_FAIL_CACHE)
	WT_STAT_CONN_TXN_PINNED_RANGE            = int(C.WT_STAT_CONN_TXN_PINNED_RANGE)
	WT_STAT_CONN_TXN_PINNED_CHECKPOINT_RANGE = int(C.WT_STAT_CONN_TXN_PINNED_CHECKPOINT_RANGE)
	WT_STAT_CONN_TXN_PINNED_SNAPSHOT_RANGE   = int(C.WT_STAT_CONN_TXN_PINNED_SNAPSHOT_RANGE)
	WT_STAT_CONN_TXN_SYNC                    = int(C.WT_STAT_CONN_TXN_SYNC)
	WT_STAT_CONN_TXN_COMMIT                  = int(C.WT_STAT_CONN_TXN_COMMIT)
	WT_STAT_CONN_TXN_ROLLBACK                = int(C.WT_STAT_CONN_TXN_ROLLBACK)
)

// Statistics for data sources
const (
	WT_STAT_DSRC_BLOOM_FALSE_POSITIVE          = int(C.WT_STAT_DSRC_BLOOM_FALSE_POSITIVE)
	WT_STAT_DSRC_BLOOM_HIT                     = int(C.WT_STAT_DSRC_BLOOM_HIT)
	WT_STAT_DSRC_BLOOM_MISS                    = int(C.WT_STAT_DSRC_BLOOM_MISS)
	WT_STAT_DSRC_BLOOM_PAGE_EVICT              = int(C.WT_STAT_DSRC_BLOOM_PAGE_EVICT)
	WT_STAT_DSRC_BLOOM_PAGE_READ               = int(C.WT_STAT_DSRC_BLOOM_PAGE_READ)
	WT_STAT_DSRC_BLOOM_COUNT                   = int(C.WT_STAT_DSRC_BLOOM_COUNT)
	WT_STAT_DSRC_LSM_CHUNK_COUNT               = int(C.WT_STAT_DSRC_LSM_CHUNK_COUNT)
	WT_STAT_DSRC_LSM_GENERATION_MAX            = int(C.WT_STAT_DSRC_LSM_GENERATION_MAX)
	WT_STAT_DSRC_LSM_LOOKUP_NO_BLOOM           = int(C.WT_STAT_DSRC_LSM_LOOKUP_NO_BLOOM)
	WT_STAT_DSRC_LSM_CHECKPOINT_THROTTLE       = int(C.WT_STAT_DSRC_LSM_CHECKPOINT_THROTTLE)
	WT_STAT_DSRC_LSM_MERGE_THROTTLE            = int(C.WT_STAT_DSRC_LSM_MERGE_THROTTLE)
	WT_STAT_DSRC_BLOOM_SIZE                    = int(C.WT_STAT_DSRC_BLOOM_SIZE)
	WT_STAT_DSRC_BLOCK_EXTENSION               = int(C.WT_STAT_DSRC_BLOCK_EXTENSION)
	WT_STAT_DSRC_BLOCK_ALLOC                   = int(C.WT_STAT_DSRC_BLOCK_ALLOC)
	WT_STAT_DSRC_BLOCK_FREE                    = int(C.WT_STAT_DSRC_BLOCK_FREE)
	WT_STAT_DSRC_BLOCK_CHECKPOINT_SIZE         = int(C.WT_STAT_DSRC_BLOCK_CHECKPOINT_SIZE)
	WT_STAT_DSRC_ALLOCATION_SIZE               = int(C.WT_STAT_DSRC_ALLOCATION_SIZE)
	WT_STAT_DSRC_BLOCK_REUSE_BYTES             = int(C.WT_STAT_DSRC_BLOCK_REUSE_BYTES)
	WT_STAT_DSRC_BLOCK_MAGIC                   = int(C.WT_STAT_DSRC_BLOCK_MAGIC)
	WT_STAT_DSRC_BLOCK_MAJOR                   = int(C.WT_STAT_DSRC_BLOCK_MAJOR)
	WT_STAT_DSRC_BLOCK_SIZE                    = int(C.WT_STAT_DSRC_BLOCK_SIZE)
	WT_STAT_DSRC_BLOCK_MINOR                   = int(C.WT_STAT_DSRC_BLOCK_MINOR)
	WT_STAT_DSRC_BTREE_CHECKPOINT_GENERATION   = int(C.WT_STAT_DSRC_BTREE_CHECKPOINT_GENERATION)
	WT_STAT_DSRC_BTREE_COLUMN_FIX              = int(C.WT_STAT_DSRC_BTREE_COLUMN_FIX)
	WT_STAT_DSRC_BTREE_COLUMN_INTERNAL         = int(C.WT_STAT_DSRC_BTREE_COLUMN_INTERNAL)
	WT_STAT_DSRC_BTREE_COLUMN_RLE              = int(C.WT_STAT_DSRC_BTREE_COLUMN_RLE)
	WT_STAT_DSRC_BTREE_COLUMN_DELETED          = int(C.WT_STAT_DSRC_BTREE_COLUMN_DELETED)
	WT_STAT_DSRC_BTREE_COLUMN_VARIABLE         = int(C.WT_STAT_DSRC_BTREE_COLUMN_VARIABLE)
	WT_STAT_DSRC_BTREE_FIXED_LEN               = int(C.WT_STAT_DSRC_BTREE_FIXED_LEN)
	WT_STAT_DSRC_BTREE_MAXINTLKEY              = int(C.WT_STAT_DSRC_BTREE_MAXINTLKEY)
	WT_STAT_DSRC_BTREE_MAXINTLPAGE             = int(C.WT_STAT_DSRC_BTREE_MAXINTLPAGE)
	WT_STAT_DSRC_BTREE_MAXLEAFKEY              = int(C.WT_STAT_DSRC_BTREE_MAXLEAFKEY)
	WT_STAT_DSRC_BTREE_MAXLEAFPAGE             = int(C.WT_STAT_DSRC_BTREE_MAXLEAFPAGE)
	WT_STAT_DSRC_BTREE_MAXLEAFVALUE            = int(C.WT_STAT_DSRC_BTREE_MAXLEAFVALUE)
	WT_STAT_DSRC_BTREE_MAXIMUM_DEPTH           = int(C.WT_STAT_DSRC_BTREE_MAXIMUM_DEPTH)
	WT_STAT_DSRC_BTREE_ENTRIES                 = int(C.WT_STAT_DSRC_BTREE_ENTRIES)
	WT_STAT_DSRC_BTREE_OVERFLOW                = int(C.WT_STAT_DSRC_BTREE_OVERFLOW)
	WT_STAT_DSRC_BTREE_COMPACT_REWRITE         = int(C.WT_STAT_DSRC_BTREE_COMPACT_REWRITE)
	WT_STAT_DSRC_BTREE_ROW_INTERNAL            = int(C.WT_STAT_DSRC_BTREE_ROW_INTERNAL)
	WT_STAT_DSRC_BTREE_ROW_LEAF                = int(C.WT_STAT_DSRC_BTREE_ROW_LEAF)
	WT_STAT_DSRC_CACHE_BYTES_READ              = int(C.WT_STAT_DSRC_CACHE_BYTES_READ)
	WT_STAT_DSRC_CACHE_BYTES_WRITE             = int(C.WT_STAT_DSRC_CACHE_BYTES_WRITE)
	WT_STAT_DSRC_CACHE_EVICTION_CHECKPOINT     = int(C.WT_STAT_DSRC_CACHE_EVICTION_CHECKPOINT)
	WT_STAT_DSRC_CACHE_EVICTION_FAIL           = int(C.WT_STAT_DSRC_CACHE_EVICTION_FAIL)
	WT_STAT_DSRC_CACHE_EVICTION_HAZARD         = int(C.WT_STAT_DSRC_CACHE_EVICTION_HAZARD)
	WT_STAT_DSRC_CACHE_INMEM_SPLITTABLE        = int(C.WT_STAT_DSRC_CACHE_INMEM_SPLITTABLE)
	WT_STAT_DSRC_CACHE_INMEM_SPLIT             = int(C.WT_STAT_DSRC_CACHE_INMEM_SPLIT)
	WT_STAT_DSRC_CACHE_EVICTION_INTERNAL       = int(C.WT_STAT_DSRC_CACHE_EVICTION_INTERNAL)
	WT_STAT_DSRC_CACHE_EVICTION_SPLIT_INTERNAL = int(C.WT_STAT_DSRC_CACHE_EVICTION_SPLIT_INTERNAL)
	WT_STAT_DSRC_CACHE_EVICTION_SPLIT_LEAF     = int(C.WT_STAT_DSRC_CACHE_EVICTION_SPLIT_LEAF)
	WT_STAT_DSRC_CACHE_EVICTION_DIRTY          = int(C.WT_STAT_DSRC_CACHE_EVICTION_DIRTY)
	WT_STAT_DSRC_CACHE_READ_OVERFLOW           = int(C.WT_STAT_DSRC_CACHE_READ_OVERFLOW)
	//	WT_STAT_DSRC_CACHE_OVERFLOW_VALUE          = int(C.WT_STAT_DSRC_CACHE_OVERFLOW_VALUE)
	WT_STAT_DSRC_CACHE_EVICTION_DEEPEN    = int(C.WT_STAT_DSRC_CACHE_EVICTION_DEEPEN)
	WT_STAT_DSRC_CACHE_WRITE_LOOKASIDE    = int(C.WT_STAT_DSRC_CACHE_WRITE_LOOKASIDE)
	WT_STAT_DSRC_CACHE_READ               = int(C.WT_STAT_DSRC_CACHE_READ)
	WT_STAT_DSRC_CACHE_READ_LOOKASIDE     = int(C.WT_STAT_DSRC_CACHE_READ_LOOKASIDE)
	WT_STAT_DSRC_CACHE_WRITE              = int(C.WT_STAT_DSRC_CACHE_WRITE)
	WT_STAT_DSRC_CACHE_WRITE_RESTORE      = int(C.WT_STAT_DSRC_CACHE_WRITE_RESTORE)
	WT_STAT_DSRC_CACHE_EVICTION_CLEAN     = int(C.WT_STAT_DSRC_CACHE_EVICTION_CLEAN)
	WT_STAT_DSRC_COMPRESS_READ            = int(C.WT_STAT_DSRC_COMPRESS_READ)
	WT_STAT_DSRC_COMPRESS_WRITE           = int(C.WT_STAT_DSRC_COMPRESS_WRITE)
	WT_STAT_DSRC_COMPRESS_WRITE_FAIL      = int(C.WT_STAT_DSRC_COMPRESS_WRITE_FAIL)
	WT_STAT_DSRC_COMPRESS_WRITE_TOO_SMALL = int(C.WT_STAT_DSRC_COMPRESS_WRITE_TOO_SMALL)
	//	WT_STAT_DSRC_COMPRESS_RAW_FAIL_TEMPORARY   = int(C.WT_STAT_DSRC_COMPRESS_RAW_FAIL_TEMPORARY)
	//	WT_STAT_DSRC_COMPRESS_RAW_FAIL             = int(C.WT_STAT_DSRC_COMPRESS_RAW_FAIL)
	//	WT_STAT_DSRC_COMPRESS_RAW_OK               = int(C.WT_STAT_DSRC_COMPRESS_RAW_OK)
	WT_STAT_DSRC_CURSOR_INSERT_BULK        = int(C.WT_STAT_DSRC_CURSOR_INSERT_BULK)
	WT_STAT_DSRC_CURSOR_CREATE             = int(C.WT_STAT_DSRC_CURSOR_CREATE)
	WT_STAT_DSRC_CURSOR_INSERT_BYTES       = int(C.WT_STAT_DSRC_CURSOR_INSERT_BYTES)
	WT_STAT_DSRC_CURSOR_REMOVE_BYTES       = int(C.WT_STAT_DSRC_CURSOR_REMOVE_BYTES)
	WT_STAT_DSRC_CURSOR_UPDATE_BYTES       = int(C.WT_STAT_DSRC_CURSOR_UPDATE_BYTES)
	WT_STAT_DSRC_CURSOR_INSERT             = int(C.WT_STAT_DSRC_CURSOR_INSERT)
	WT_STAT_DSRC_CURSOR_NEXT               = int(C.WT_STAT_DSRC_CURSOR_NEXT)
	WT_STAT_DSRC_CURSOR_PREV               = int(C.WT_STAT_DSRC_CURSOR_PREV)
	WT_STAT_DSRC_CURSOR_REMOVE             = int(C.WT_STAT_DSRC_CURSOR_REMOVE)
	WT_STAT_DSRC_CURSOR_RESET              = int(C.WT_STAT_DSRC_CURSOR_RESET)
	WT_STAT_DSRC_CURSOR_RESTART            = int(C.WT_STAT_DSRC_CURSOR_RESTART)
	WT_STAT_DSRC_CURSOR_SEARCH             = int(C.WT_STAT_DSRC_CURSOR_SEARCH)
	WT_STAT_DSRC_CURSOR_SEARCH_NEAR        = int(C.WT_STAT_DSRC_CURSOR_SEARCH_NEAR)
	WT_STAT_DSRC_CURSOR_TRUNCATE           = int(C.WT_STAT_DSRC_CURSOR_TRUNCATE)
	WT_STAT_DSRC_CURSOR_UPDATE             = int(C.WT_STAT_DSRC_CURSOR_UPDATE)
	WT_STAT_DSRC_REC_DICTIONARY            = int(C.WT_STAT_DSRC_REC_DICTIONARY)
	WT_STAT_DSRC_REC_PAGE_DELETE_FAST      = int(C.WT_STAT_DSRC_REC_PAGE_DELETE_FAST)
	WT_STAT_DSRC_REC_SUFFIX_COMPRESSION    = int(C.WT_STAT_DSRC_REC_SUFFIX_COMPRESSION)
	WT_STAT_DSRC_REC_MULTIBLOCK_INTERNAL   = int(C.WT_STAT_DSRC_REC_MULTIBLOCK_INTERNAL)
	WT_STAT_DSRC_REC_OVERFLOW_KEY_INTERNAL = int(C.WT_STAT_DSRC_REC_OVERFLOW_KEY_INTERNAL)
	WT_STAT_DSRC_REC_PREFIX_COMPRESSION    = int(C.WT_STAT_DSRC_REC_PREFIX_COMPRESSION)
	WT_STAT_DSRC_REC_MULTIBLOCK_LEAF       = int(C.WT_STAT_DSRC_REC_MULTIBLOCK_LEAF)
	WT_STAT_DSRC_REC_OVERFLOW_KEY_LEAF     = int(C.WT_STAT_DSRC_REC_OVERFLOW_KEY_LEAF)
	WT_STAT_DSRC_REC_MULTIBLOCK_MAX        = int(C.WT_STAT_DSRC_REC_MULTIBLOCK_MAX)
	WT_STAT_DSRC_REC_OVERFLOW_VALUE        = int(C.WT_STAT_DSRC_REC_OVERFLOW_VALUE)
	WT_STAT_DSRC_REC_PAGE_MATCH            = int(C.WT_STAT_DSRC_REC_PAGE_MATCH)
	WT_STAT_DSRC_REC_PAGES                 = int(C.WT_STAT_DSRC_REC_PAGES)
	WT_STAT_DSRC_REC_PAGES_EVICTION        = int(C.WT_STAT_DSRC_REC_PAGES_EVICTION)
	WT_STAT_DSRC_REC_PAGE_DELETE           = int(C.WT_STAT_DSRC_REC_PAGE_DELETE)
	WT_STAT_DSRC_SESSION_COMPACT           = int(C.WT_STAT_DSRC_SESSION_COMPACT)
	//	WT_STAT_DSRC_SESSION_CURSOR_OPEN           = int(C.WT_STAT_DSRC_SESSION_CURSOR_OPEN)
	WT_STAT_DSRC_TXN_UPDATE_CONFLICT = int(C.WT_STAT_DSRC_TXN_UPDATE_CONFLICT)
)

// Statistics for join cursors
const (
	//	WT_STAT_JOIN_ACCESSES             = int(C.WT_STAT_JOIN_ACCESSES)
	//	WT_STAT_JOIN_ACTUAL_COUNT         = int(C.WT_STAT_JOIN_ACTUAL_COUNT)
	WT_STAT_JOIN_BLOOM_FALSE_POSITIVE = int(C.WT_STAT_JOIN_BLOOM_FALSE_POSITIVE)
)

// Log record and operation types
const (
	WT_LOGOP_INVALID      = int(C.WT_LOGOP_INVALID)
	WT_LOGREC_CHECKPOINT  = int(C.WT_LOGREC_CHECKPOINT)
	WT_LOGREC_COMMIT      = int(C.WT_LOGREC_COMMIT)
	WT_LOGREC_FILE_SYNC   = int(C.WT_LOGREC_FILE_SYNC)
	WT_LOGREC_MESSAGE     = int(C.WT_LOGREC_MESSAGE)
	WT_LOGOP_COL_PUT      = int(C.WT_LOGOP_COL_PUT)
	WT_LOGOP_COL_REMOVE   = int(C.WT_LOGOP_COL_REMOVE)
	WT_LOGOP_COL_TRUNCATE = int(C.WT_LOGOP_COL_TRUNCATE)
	WT_LOGOP_ROW_PUT      = int(C.WT_LOGOP_ROW_PUT)
	WT_LOGOP_ROW_REMOVE   = int(C.WT_LOGOP_ROW_REMOVE)
	WT_LOGOP_ROW_TRUNCATE = int(C.WT_LOGOP_ROW_TRUNCATE)
)
