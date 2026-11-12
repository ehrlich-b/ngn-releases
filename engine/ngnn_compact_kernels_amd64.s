//go:build amd64 && amd64.v3 && !purego && !race

#include "textflag.h"

// NGN-owned compact accumulator kernels. Word operations wrap, never saturate.

TEXT ·ngnn16AddRow(SB), NOSPLIT, $0-48
	MOVQ acc_base+0(FP), DI
	MOVQ acc_base+0(FP), SI
	MOVQ row_base+24(FP), R8
	MOVQ row_len+32(FP), CX
CompactAddRow_loop:
	VMOVDQU (SI), Y0
	VMOVDQU (R8), Y1
	VPADDW Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $32, R8
	SUBQ $16, CX
	JNZ CompactAddRow_loop
	VZEROUPPER
	RET

TEXT ·ngnn16SubRow(SB), NOSPLIT, $0-48
	MOVQ acc_base+0(FP), DI
	MOVQ acc_base+0(FP), SI
	MOVQ row_base+24(FP), R8
	MOVQ row_len+32(FP), CX
CompactSubRow_loop:
	VMOVDQU (SI), Y0
	VMOVDQU (R8), Y1
	VPSUBW Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $32, R8
	SUBQ $16, CX
	JNZ CompactSubRow_loop
	VZEROUPPER
	RET

TEXT ·ngnn16MoveRows(SB), NOSPLIT, $0-96
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ remove_base+48(FP), R8
	MOVQ add_base+72(FP), R9
	MOVQ add_len+80(FP), CX
CompactMoveRows_loop:
	VMOVDQU (SI), Y0
	VMOVDQU (R8), Y1
	VPSUBW Y1, Y0, Y0
	VMOVDQU (R9), Y1
	VPADDW Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $32, R8
	ADDQ $32, R9
	SUBQ $16, CX
	JNZ CompactMoveRows_loop
	VZEROUPPER
	RET

TEXT ·ngnn16MovePair(SB), NOSPLIT, $0-144
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ removeUs_base+48(FP), R8
	MOVQ addUs_base+72(FP), R9
	MOVQ removeThem_base+96(FP), R10
	MOVQ addThem_base+120(FP), R11
	MOVQ addUs_len+80(FP), CX
	LEAQ (DI)(CX*2), R12
	LEAQ (SI)(CX*2), R13
CompactMovePair_loop:
	VMOVDQU (SI), Y0
	VMOVDQU (R13), Y2
	VMOVDQU (R8), Y1
	VMOVDQU (R10), Y3
	VPSUBW Y1, Y0, Y0
	VPSUBW Y3, Y2, Y2
	VMOVDQU (R9), Y1
	VMOVDQU (R11), Y3
	VPADDW Y1, Y0, Y0
	VPADDW Y3, Y2, Y2
	VMOVDQU Y0, (DI)
	VMOVDQU Y2, (R12)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $32, R12
	ADDQ $32, R13
	ADDQ $32, R8
	ADDQ $32, R9
	ADDQ $32, R10
	ADDQ $32, R11
	SUBQ $16, CX
	JNZ CompactMovePair_loop
	VZEROUPPER
	RET

TEXT ·ngnn16CastleRows(SB), NOSPLIT, $0-144
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ removeKing_base+48(FP), R8
	MOVQ removeRook_base+72(FP), R9
	MOVQ addKing_base+96(FP), R10
	MOVQ addRook_base+120(FP), R11
	MOVQ addRook_len+128(FP), CX
CompactCastleRows_loop:
	VMOVDQU (SI), Y0
	VMOVDQU (R8), Y1
	VPSUBW Y1, Y0, Y0
	VMOVDQU (R9), Y1
	VPSUBW Y1, Y0, Y0
	VMOVDQU (R10), Y1
	VPADDW Y1, Y0, Y0
	VMOVDQU (R11), Y1
	VPADDW Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $32, R8
	ADDQ $32, R9
	ADDQ $32, R10
	ADDQ $32, R11
	SUBQ $16, CX
	JNZ CompactCastleRows_loop
	VZEROUPPER
	RET

TEXT ·ngnn16CapturePair(SB), NOSPLIT, $0-192
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ removeUs_base+48(FP), R8
	MOVQ captureUs_base+72(FP), R9
	MOVQ addUs_base+96(FP), R10
	MOVQ removeThem_base+120(FP), R11
	MOVQ captureThem_base+144(FP), R12
	MOVQ addThem_base+168(FP), R13
	MOVQ addUs_len+104(FP), CX
	LEAQ (DI)(CX*2), BX
	LEAQ (SI)(CX*2), DX
CompactCapturePair_loop:
	VMOVDQU (SI), Y0
	VMOVDQU (DX), Y2
	VPSUBW (R8), Y0, Y0
	VPSUBW (R11), Y2, Y2
	VPSUBW (R9), Y0, Y0
	VPSUBW (R12), Y2, Y2
	VPADDW (R10), Y0, Y0
	VPADDW (R13), Y2, Y2
	VMOVDQU Y0, (DI)
	VMOVDQU Y2, (BX)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $32, BX
	ADDQ $32, DX
	ADDQ $32, R8
	ADDQ $32, R9
	ADDQ $32, R10
	ADDQ $32, R11
	ADDQ $32, R12
	ADDQ $32, R13
	SUBQ $16, CX
	JNZ CompactCapturePair_loop
	VZEROUPPER
	RET

TEXT ·ngnn16OutputSmall(SB), NOSPLIT, $0-80
	MOVQ us_base+0(FP), SI
	MOVQ them_base+24(FP), DI
	MOVQ weights_base+48(FP), R8
	MOVQ us_len+8(FP), CX
	LEAQ (R8)(CX*2), R9
	VPXOR Y5, Y5, Y5
	VPXOR Y6, Y6, Y6
	VPXOR Y7, Y7, Y7
	MOVL $255, AX
	MOVD AX, X4
	VPBROADCASTW X4, Y4
CompactOutputSmall_loop:
	VMOVDQU (SI), Y2
	VPMAXSW Y5, Y2, Y2
	VPMINSW Y4, Y2, Y2
	VPMULLW (R8), Y2, Y3
	VPMADDWD Y2, Y3, Y3
	VPADDD Y3, Y6, Y6
	VMOVDQU (DI), Y2
	VPMAXSW Y5, Y2, Y2
	VPMINSW Y4, Y2, Y2
	VPMULLW (R9), Y2, Y3
	VPMADDWD Y2, Y3, Y3
	VPADDD Y3, Y7, Y7
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $32, R8
	ADDQ $32, R9
	SUBQ $16, CX
	JNZ CompactOutputSmall_loop
	VPMOVSXDQ X6, Y0
	VEXTRACTI128 $1, Y6, X1
	VPMOVSXDQ X1, Y1
	VPMOVSXDQ X7, Y2
	VEXTRACTI128 $1, Y7, X3
	VPMOVSXDQ X3, Y3
	VPADDQ Y1, Y0, Y0
	VPADDQ Y3, Y2, Y2
	VPADDQ Y2, Y0, Y0
	VEXTRACTI128 $1, Y0, X1
	VPADDQ X1, X0, X0
	VPSHUFD $0x4e, X0, X1
	VPADDQ X1, X0, X0
	MOVQ X0, AX
	MOVQ AX, ret+72(FP)
	VZEROUPPER
	RET
